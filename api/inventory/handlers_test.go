package inventory

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/inventorylevel"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// callAdjust drives AdjustStock over a real request wired so the handler's
// datastore.New(c.Context()) resolves to namespace ns. body is the raw
// adjustment JSON. Returns the response status and body.
func callAdjust(t *testing.T, ns, levelID, body string) (int, []byte) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	seed := func(c *zip.Ctx) error {
		c.SetContext(nscontext.WithNamespace(context.Background(), ns))
		return c.Next()
	}
	app.Post("/inventory/level/:inventorylevelid/adjust", seed, AdjustStock)

	req := httptest.NewRequest(http.MethodPost, "/inventory/level/"+levelID+"/adjust", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Fiber().Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// seedLevel creates an InventoryLevel in namespace ns.
func seedLevel(t *testing.T, ns string, stocked, reserved int) *inventorylevel.InventoryLevel {
	t.Helper()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))
	lvl := inventorylevel.New(db)
	lvl.InventoryItemId = "item-1"
	lvl.LocationId = "loc-1"
	lvl.StockedQuantity = stocked
	lvl.ReservedQuantity = reserved
	if err := lvl.Create(); err != nil {
		t.Fatalf("seed level: %v", err)
	}
	return lvl
}

func reloadLevel(t *testing.T, ns, id string) *inventorylevel.InventoryLevel {
	t.Helper()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))
	lvl := inventorylevel.New(db)
	if err := lvl.GetById(id); err != nil {
		t.Fatalf("reload level: %v", err)
	}
	return lvl
}

// TestAdjust_OversellRefused proves the oversell guard: a reservation that would
// drive available (stocked − reserved) below zero is refused with 409 and does
// NOT mutate the stored level. Reserving up to exactly available succeeds; one
// more is refused.
func TestAdjust_OversellRefused(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	const ns = ""
	lvl := seedLevel(t, ns, 5, 0) // 5 in stock, none reserved → available 5
	id := lvl.Id()

	// Reserve 20 against 5 available → refused, level untouched.
	status, body := callAdjust(t, ns, id, `{"reservedQuantity":20}`)
	if status != 409 {
		t.Fatalf("oversell reserve 20 status = %d, want 409; body=%s", status, body)
	}
	after := reloadLevel(t, ns, id)
	if after.AvailableQuantity() != 5 {
		t.Fatalf("after refused oversell: available = %d, want 5 (unchanged)", after.AvailableQuantity())
	}
	if after.ReservedQuantity != 0 {
		t.Fatalf("after refused oversell: reserved = %d, want 0 (not mutated)", after.ReservedQuantity)
	}

	// Reserve exactly 5 → allowed, available 0.
	status, body = callAdjust(t, ns, id, `{"reservedQuantity":5}`)
	if status != 200 {
		t.Fatalf("reserve 5 status = %d, want 200; body=%s", status, body)
	}
	after = reloadLevel(t, ns, id)
	if after.AvailableQuantity() != 0 {
		t.Fatalf("after reserve 5: available = %d, want 0", after.AvailableQuantity())
	}

	// One more unit → refused, still available 0.
	status, body = callAdjust(t, ns, id, `{"reservedQuantity":1}`)
	if status != 409 {
		t.Fatalf("reserve 1 more status = %d, want 409; body=%s", status, body)
	}
	after = reloadLevel(t, ns, id)
	if after.AvailableQuantity() != 0 {
		t.Fatalf("after refused reserve 1: available = %d, want 0 (unchanged)", after.AvailableQuantity())
	}
}

// TestAdjust_ReserveFulfillReleaseRestock proves the legitimate accounting stays
// green: reserving, fulfilling (ship: stock− and reserved−), releasing a
// reservation (reserved−), and restocking (stock+) all succeed and keep
// available ≥ 0.
func TestAdjust_ReserveFulfillReleaseRestock(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	const ns = ""
	lvl := seedLevel(t, ns, 10, 0) // available 10
	id := lvl.Id()

	assert := func(step string, wantStocked, wantReserved, wantAvail int) {
		l := reloadLevel(t, ns, id)
		if l.StockedQuantity != wantStocked || l.ReservedQuantity != wantReserved || l.AvailableQuantity() != wantAvail {
			t.Fatalf("%s: stocked=%d reserved=%d available=%d; want stocked=%d reserved=%d available=%d",
				step, l.StockedQuantity, l.ReservedQuantity, l.AvailableQuantity(), wantStocked, wantReserved, wantAvail)
		}
	}

	// Reserve 4 → available 6.
	if s, b := callAdjust(t, ns, id, `{"reservedQuantity":4}`); s != 200 {
		t.Fatalf("reserve 4 = %d; %s", s, b)
	}
	assert("reserve 4", 10, 4, 6)

	// Fulfill 4 (ship the goods: stock−4, reserved−4) → available 6.
	if s, b := callAdjust(t, ns, id, `{"stockedQuantity":-4,"reservedQuantity":-4}`); s != 200 {
		t.Fatalf("fulfill 4 = %d; %s", s, b)
	}
	assert("fulfill 4", 6, 0, 6)

	// Reserve 6 → available 0.
	if s, b := callAdjust(t, ns, id, `{"reservedQuantity":6}`); s != 200 {
		t.Fatalf("reserve 6 = %d; %s", s, b)
	}
	assert("reserve 6", 6, 6, 0)

	// Release the 6-unit reservation (reserved−6) → available 6.
	if s, b := callAdjust(t, ns, id, `{"reservedQuantity":-6}`); s != 200 {
		t.Fatalf("release 6 = %d; %s", s, b)
	}
	assert("release 6", 6, 0, 6)

	// Restock incoming +10 → available 16.
	if s, b := callAdjust(t, ns, id, `{"stockedQuantity":10}`); s != 200 {
		t.Fatalf("restock 10 = %d; %s", s, b)
	}
	assert("restock 10", 16, 0, 16)
}
