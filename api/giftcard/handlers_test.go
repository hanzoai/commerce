package giftcard

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/datastore"
	giftcardModel "github.com/hanzoai/commerce/models/giftcard"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// callGiftcard drives money handler h over a real request wired so
// middleware.GetOrganization(c) + org.Namespaced(c.Context()) resolve to the ae SQLite
// datastore in org `ns`'s namespace — the exact production plumbing (org name IS
// the namespace). admin selects whether the injected IAM claim (the one the
// gateway/EdgeAuth would mint) authorizes the admin-gated money action. Returns
// the response status and body.
func callGiftcard(t *testing.T, ns string, admin bool, cardID string, body []byte, h zip.Handler) (int, []byte) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	seed := func(c *zip.Ctx) error {
		org := &organization.Organization{}
		org.Name = ns
		c.Locals("organization", org)
		c.SetContext(nscontext.WithNamespace(context.Background(), ns))
		c.Locals("iam_authenticated", true)
		c.Locals("iam_claims", &auth.IAMClaims{Owner: ns, IsAdmin: admin})
		return c.Next()
	}
	app.Post("/giftcard/:giftcardid", seed, h)

	req := httptest.NewRequest(http.MethodPost, "/giftcard/"+cardID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Fiber().Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// issueCard seeds a gift card in org `ns`.
func issueCard(t *testing.T, base context.Context, ns, code string, cents int64) *giftcardModel.GiftCard {
	t.Helper()
	db := datastore.New(nscontext.WithNamespace(base, ns))
	g := giftcardModel.New(db)
	g.Code = code
	g.InitialBalanceCents = currency.Cents(cents)
	g.Currency = "usd"
	if err := g.Create(); err != nil {
		t.Fatalf("seed card: %v", err)
	}
	return g
}

// TestRedeem_HTTP_IdempotentReplay proves the money path over the wire: two
// POSTs with the same idempotency key return 200 and debit exactly once.
func TestRedeem_HTTP_IdempotentReplay(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	g := issueCard(t, context.Background(), "acme", "GIFT-HTTP", 5000)

	body, _ := json.Marshal(redeemRequest{AmountCents: 1500, Currency: "usd", IdempotencyKey: "http_idem_1"})

	do := func() (int, redeemResponse) {
		code, b := callGiftcard(t, "acme", true, g.Id(), body, Redeem)
		var resp redeemResponse
		_ = json.Unmarshal(b, &resp)
		return code, resp
	}

	code1, resp1 := do()
	if code1 != 200 {
		t.Fatalf("redeem 1 status = %d", code1)
	}
	if resp1.BalanceCents != 3500 {
		t.Fatalf("balance after redeem 1 = %d, want 3500", resp1.BalanceCents)
	}

	code2, resp2 := do()
	if code2 != 200 {
		t.Fatalf("redeem replay status = %d", code2)
	}
	if resp2.BalanceCents != 3500 {
		t.Fatalf("balance after replay = %d, want 3500 (idempotent — no second debit)", resp2.BalanceCents)
	}
	if resp1.Redemption.Id() != resp2.Redemption.Id() {
		t.Fatalf("replay produced a different redemption: %s vs %s", resp1.Redemption.Id(), resp2.Redemption.Id())
	}
}

// TestRedeem_HTTP_InsufficientFunds proves over-redeem maps to 402, not 500.
func TestRedeem_HTTP_InsufficientFunds(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	g := issueCard(t, context.Background(), "acme", "GIFT-HTTP-OVER", 1000)

	body, _ := json.Marshal(redeemRequest{AmountCents: 5000, Currency: "usd", IdempotencyKey: "http_over"})
	code, b := callGiftcard(t, "acme", true, g.Id(), body, Redeem)

	if code != 402 {
		t.Fatalf("over-redeem status = %d, want 402; body=%s", code, b)
	}
}

// TestRedeem_Void_NonAdmin_403 proves the HIGH-4 fix: the gift-card money
// subroutes (redeem, void) — which util/rest.Route did NOT wrap with the auth
// middleware and TokenRequired(Admin) no-ops on the IAM path — now reject a
// non-admin caller with 403, BEFORE any card mutation. A merchant staffer with a
// valid non-admin session must not be able to drain gift cards.
func TestRedeem_Void_NonAdmin_403(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	g := issueCard(t, context.Background(), "acme", "GIFT-NOADMIN", 5000)

	cases := []struct {
		name    string
		handler zip.Handler
		body    []byte
	}{
		{"redeem", Redeem, mustJSON(redeemRequest{AmountCents: 100, Currency: "usd", IdempotencyKey: "na"})},
		{"void", Void, mustJSON(voidRequest{RedemptionId: "any"})},
	}
	for _, tcase := range cases {
		t.Run(tcase.name, func(t *testing.T) {
			code, b := callGiftcard(t, "acme", false, g.Id(), tcase.body, tcase.handler) // authenticated, NOT admin
			if code != 403 {
				t.Fatalf("%s as non-admin = %d, want 403 (money subroute must gate admin); body=%s", tcase.name, code, b)
			}
		})
	}

	// Balance must be untouched — the gate fired before any redemption.
	adb := datastore.New(nscontext.WithNamespace(context.Background(), "acme"))
	after := giftcardModel.New(adb)
	if err := after.GetById(g.Id()); err != nil {
		t.Fatalf("reload card: %v", err)
	}
	bal, err := giftcardModel.BalanceCents(adb, after)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	if bal != currency.Cents(5000) {
		t.Fatalf("balance = %d after refused non-admin redeem, want 5000 (no debit)", bal)
	}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// TestRedeem_HTTP_CrossTenant404 proves a card in org acme is a 404 for org beta.
func TestRedeem_HTTP_CrossTenant404(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	g := issueCard(t, context.Background(), "acme", "GIFT-HTTP-ISO", 5000)

	body, _ := json.Marshal(redeemRequest{AmountCents: 100, Currency: "usd", IdempotencyKey: "http_iso"})
	code, b := callGiftcard(t, "beta", true, g.Id(), body, Redeem) // caller is org beta, card belongs to acme

	if code != 404 {
		t.Fatalf("cross-tenant redeem status = %d, want 404 (tenant isolation); body=%s", code, b)
	}
}
