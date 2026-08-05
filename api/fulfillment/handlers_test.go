package fulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/fulfillmentmodel"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// fulfillmentAPI wires the real Ship/Cancel handlers behind a seed middleware
// that binds org `ns`, so the state-machine transitions run their real
// org.Namespaced(...) datastore path.
type fulfillmentAPI struct {
	app *zip.App
	db  *datastore.Datastore
}

func newFulfillmentAPI(ns string) *fulfillmentAPI {
	base := nscontext.WithNamespace(context.Background(), ns)
	db := datastore.New(base)
	org := organization.New(db)
	org.Name = ns

	app := zip.New(zip.Config{DisableStartupMessage: true})
	seed := func(c *zip.Ctx) error {
		c.SetContext(base)
		c.Locals("organization", org)
		return c.Next()
	}
	app.Post("/:fulfillmentid/ship", seed, Ship)
	app.Post("/:fulfillmentid/cancel", seed, Cancel)

	return &fulfillmentAPI{app: app, db: db}
}

func (a *fulfillmentAPI) do(t *testing.T, path string, body any) (int, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		r = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(http.MethodPost, path, r)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.app.Test(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func (a *fulfillmentAPI) seedFulfillment(t *testing.T) *fulfillmentmodel.Fulfillment {
	t.Helper()
	f := fulfillmentmodel.New(a.db)
	f.OrderId = "order-1"
	if err := f.Create(); err != nil {
		t.Fatalf("create fulfillment: %v", err)
	}
	return f
}

func (a *fulfillmentAPI) reload(t *testing.T, id string) *fulfillmentmodel.Fulfillment {
	t.Helper()
	f := fulfillmentmodel.New(a.db)
	if err := f.GetById(id); err != nil {
		t.Fatalf("reload fulfillment: %v", err)
	}
	return f
}

// TestShip marks a fulfillment shipped (sets ShippedAt) and appends the
// tracking labels from the request body.
func TestShip(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newFulfillmentAPI("acme")
	f := api.seedFulfillment(t)

	code, body := api.do(t, "/"+f.Id()+"/ship", map[string]any{
		"labels": []map[string]any{{"trackingNumber": "1Z999", "trackingUrl": "https://track/1Z999"}},
	})
	if code != http.StatusOK {
		t.Fatalf("ship status = %d, body=%s", code, body)
	}
	got := api.reload(t, f.Id())
	if got.ShippedAt == nil {
		t.Fatalf("ShippedAt is nil after ship")
	}
	if len(got.Labels) != 1 || got.Labels[0].TrackingNumber != "1Z999" {
		t.Fatalf("labels = %+v, want one 1Z999 label", got.Labels)
	}
}

// TestCancel marks a fulfillment canceled (sets CanceledAt).
func TestCancel(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newFulfillmentAPI("acme")
	f := api.seedFulfillment(t)

	code, body := api.do(t, "/"+f.Id()+"/cancel", nil)
	if code != http.StatusOK {
		t.Fatalf("cancel status = %d, body=%s", code, body)
	}
	got := api.reload(t, f.Id())
	if got.CanceledAt == nil {
		t.Fatalf("CanceledAt is nil after cancel")
	}
}

// TestShip_AfterCancelRefused: a canceled fulfillment cannot be shipped — the
// terminal-state guard on the state machine.
func TestShip_AfterCancelRefused(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newFulfillmentAPI("acme")
	f := api.seedFulfillment(t)

	if code, _ := api.do(t, "/"+f.Id()+"/cancel", nil); code != http.StatusOK {
		t.Fatalf("precondition cancel = %d, want 200", code)
	}
	code, body := api.do(t, "/"+f.Id()+"/ship", nil)
	if code != http.StatusBadRequest {
		t.Fatalf("ship after cancel = %d, want 400; body=%s", code, body)
	}
	// Still not shipped.
	got := api.reload(t, f.Id())
	if got.ShippedAt != nil {
		t.Fatalf("ShippedAt set on a canceled fulfillment")
	}
}

// TestCancel_TwiceRefused: canceling an already-canceled fulfillment is a 400
// (idempotency guard rejects the double transition).
func TestCancel_TwiceRefused(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newFulfillmentAPI("acme")
	f := api.seedFulfillment(t)

	if code, _ := api.do(t, "/"+f.Id()+"/cancel", nil); code != http.StatusOK {
		t.Fatalf("first cancel = %d, want 200", code)
	}
	code, body := api.do(t, "/"+f.Id()+"/cancel", nil)
	if code != http.StatusBadRequest {
		t.Fatalf("second cancel = %d, want 400; body=%s", code, body)
	}
}

// TestShip_UnknownRefused / TestCancel_UnknownRefused: transitions on a
// nonexistent fulfillment are 404.
func TestShip_UnknownRefused(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newFulfillmentAPI("acme")
	if code, _ := api.do(t, "/nope/ship", nil); code != http.StatusNotFound {
		t.Fatalf("ship unknown = %d, want 404", code)
	}
}

func TestCancel_UnknownRefused(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newFulfillmentAPI("acme")
	if code, _ := api.do(t, "/nope/cancel", nil); code != http.StatusNotFound {
		t.Fatalf("cancel unknown = %d, want 404", code)
	}
}
