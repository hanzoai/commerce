package coupon

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
	couponModel "github.com/hanzoai/commerce/models/coupon"
	"github.com/hanzoai/commerce/models/creditgrant"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// callRedeem drives redeemCoupon over a real request wired so the handler's
// datastore.New(c.Context()) resolves to namespace ns and the authenticated
// principal is uid (the local middleware.TokenRequired would set). Returns the
// response status and body.
func callRedeem(t *testing.T, ns, uid, code string) (int, []byte) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	seed := func(c *zip.Ctx) error {
		c.Locals("userId", uid)
		c.SetContext(nscontext.WithNamespace(context.Background(), ns))
		return c.Next()
	}
	app.Post("/coupon/redeem", seed, redeemCoupon)

	body, _ := json.Marshal(map[string]string{"code": code})
	req := httptest.NewRequest(http.MethodPost, "/coupon/redeem", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// seedCoupon creates a generic flat-credit coupon in namespace ns.
func seedCoupon(t *testing.T, ns, code string, amount int) *couponModel.Coupon {
	t.Helper()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))
	cpn := couponModel.New(db)
	cpn.Name = "Test Credit"
	cpn.Type = couponModel.Flat
	cpn.Code_ = code
	cpn.Amount = amount
	cpn.Enabled = true
	cpn.Limit = 0 // unlimited total redemptions; the once-per-user guard is separate
	if err := cpn.Create(); err != nil {
		t.Fatalf("seed coupon: %v", err)
	}
	return cpn
}

// grantsFor returns the credit grants held by uid in namespace ns.
func grantsFor(t *testing.T, ns, uid string) []creditgrant.CreditGrant {
	t.Helper()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))
	grants := make([]creditgrant.CreditGrant, 0)
	if _, err := creditgrant.Query(db).Filter("UserId=", uid).GetAll(&grants); err != nil {
		t.Fatalf("query grants: %v", err)
	}
	return grants
}

func totalRemaining(grants []creditgrant.CreditGrant) int64 {
	var sum int64
	for _, g := range grants {
		sum += g.RemainingCents
	}
	return sum
}

// TestRedeem_DoubleRedeemRefused proves the money-critical once-per-user guard:
// a first redeem grants credit and increments Used; a SECOND redeem by the SAME
// user for the SAME coupon is refused, mints no second grant, and leaves the
// balance unchanged. This is the fix for the double-mint bug where the guard
// queried the grant by Tags="coupon:CODE" while the grant was written
// Tags="promo,coupon:CODE" — an exact-string Tags= filter that never matched.
func TestRedeem_DoubleRedeemRefused(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	const (
		ns   = "" // root: platform coupons live and are redeemed in the root namespace
		uid  = "u-double"
		code = "SAVEDBL"
	)
	cpn := seedCoupon(t, ns, code, 500)

	// First redeem: 200, one grant of 500, Used incremented to 1.
	code1, body1 := callRedeem(t, ns, uid, code)
	if code1 != 200 {
		t.Fatalf("first redeem status = %d, want 200; body=%s", code1, body1)
	}
	g1 := grantsFor(t, ns, uid)
	if len(g1) != 1 {
		t.Fatalf("after first redeem: %d grants, want 1", len(g1))
	}
	if got := totalRemaining(g1); got != 500 {
		t.Fatalf("after first redeem: balance %d, want 500", got)
	}
	reloaded := couponModel.New(datastore.New(nscontext.WithNamespace(context.Background(), ns)))
	if err := reloaded.GetById(cpn.Id()); err != nil {
		t.Fatalf("reload coupon: %v", err)
	}
	if reloaded.Used != 1 {
		t.Fatalf("after first redeem: Used = %d, want 1", reloaded.Used)
	}

	// Second redeem by the SAME user: refused, NO second grant, balance unchanged.
	code2, body2 := callRedeem(t, ns, uid, code)
	if code2 != 400 {
		t.Fatalf("second redeem status = %d, want 400 (already redeemed); body=%s", code2, body2)
	}
	g2 := grantsFor(t, ns, uid)
	if len(g2) != 1 {
		t.Fatalf("after refused second redeem: %d grants, want 1 (no second mint)", len(g2))
	}
	if got := totalRemaining(g2); got != 500 {
		t.Fatalf("after refused second redeem: balance %d, want 500 (unchanged)", got)
	}
	reloaded2 := couponModel.New(datastore.New(nscontext.WithNamespace(context.Background(), ns)))
	if err := reloaded2.GetById(cpn.Id()); err != nil {
		t.Fatalf("reload coupon: %v", err)
	}
	if reloaded2.Used != 1 {
		t.Fatalf("after refused second redeem: Used = %d, want 1 (not incremented again)", reloaded2.Used)
	}
}

// TestRedeem_CrossNamespace404 proves the deliberate root-namespace lookup keeps
// an org-namespaced coupon invisible to the root redeem path: the coupon exists
// only in org "acme", so a root redeem 404s and mints NOTHING. (Redeem must stay
// on datastore.New — the root namespace — so an org admin's self-authored coupon
// cannot be self-minted; see the guard comment in coupon.go.)
func TestRedeem_CrossNamespace404(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	const (
		uid  = "u-iso"
		code = "ISO404"
	)
	seedCoupon(t, "acme", code, 500) // coupon lives ONLY in org acme

	status, body := callRedeem(t, "", uid, code) // root redeem cannot see it
	if status != 404 {
		t.Fatalf("cross-namespace redeem status = %d, want 404; body=%s", status, body)
	}
	if g := grantsFor(t, "", uid); len(g) != 0 {
		t.Fatalf("cross-namespace redeem minted %d grants, want 0", len(g))
	}
}
