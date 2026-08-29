package billing

// The auto-recharge wire, asserted as BYTES.
//
// GET /v1/billing/recharge answers in two shapes that differ by one key: with no
// row it omits lastRechargedAt entirely, and with a row it includes it even when
// the org has never been recharged and the value is empty. That is the quirk the
// view's Stored field exists to carry, and a test reading the response as a
// decoded map cannot see it — an absent key and an empty string both arrive as
// the zero value. Sorted map keys are load-bearing for the same reason: the
// handler renders through a map because encoding/json sorts them, so serialising
// the view struct instead would reorder every field of a response clients have
// parsed since it shipped. Both facts are only visible in the bytes.

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/autorecharge"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/test/ae"
)

// readRecharge drives GET /v1/billing/recharge for org against ctx's datastore
// and returns the response body verbatim.
func readRecharge(t *testing.T, org *organization.Organization, ctx context.Context) string {
	t.Helper()
	seed := func(c *zip.Ctx) {
		c.Locals("organization", org)
		c.SetContext(ctx)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/billing/recharge", nil)
	resp := driveSeeded(seed, "/v1/billing/recharge", req, GetAutoRecharge)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

// TestReadAutoRechargeWire_NoRowOmitsLastRecharged: an org that never set the
// rule reads as the disabled one — and the response carries NO lastRechargedAt
// key at all.
func TestReadAutoRechargeWire_NoRowOmitsLastRecharged(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("recharge-norow")

	got := readRecharge(t, org, ctx)
	want := `{"amountCents":0,"currency":"usd","enabled":false,"thresholdCents":0,"userId":"recharge-norow"}`
	if got != want {
		t.Fatalf("no-row body:\n got %s\nwant %s", got, want)
	}
}

// TestReadAutoRechargeWire_StoredRowKeepsEmptyLastRecharged: a stored row that
// has never fired carries lastRechargedAt as an EMPTY STRING. The key's presence
// is the whole point — it is how a reader knows the rule was configured, and it
// is what would vanish if the absent-row default were reused for a real row.
func TestReadAutoRechargeWire_StoredRowKeepsEmptyLastRecharged(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("recharge-row")

	db := datastore.New(org.Namespaced(ctx))
	cfg := autorecharge.New(db)
	cfg.UserId = org.Name
	cfg.Enabled = true
	cfg.ThresholdCents = 1000
	cfg.AmountCents = 5000
	cfg.Currency = "usd"
	if err := cfg.Create(); err != nil {
		t.Fatalf("seed auto-recharge row: %v", err)
	}

	got := readRecharge(t, org, ctx)
	want := `{"amountCents":5000,"currency":"usd","enabled":true,"lastRechargedAt":"","thresholdCents":1000,"userId":"recharge-row"}`
	if got != want {
		t.Fatalf("stored-row body:\n got %s\nwant %s", got, want)
	}
}
