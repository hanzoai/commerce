package billing

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/billing/tier"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/creditgrant"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
)

// Who may name whom as the subject of a subscription, and whose money pays.

func TestSquareSandbox_SubscribeWithCardNonceOk(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := moneyOrg("acme-square-sandbox")
	m := squareMock("cust_sandbox", "ccof_sandbox", "sqpay_sandbox_123")
	withFakeSquare(t, m)

	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:card-nonce-ok","planId":"team","quantity":2}`, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", resp.StatusCode, bodyOf(resp))
	}
	out := jsonBody(t, resp)
	if out["amountCents"].(float64) != 5000 {
		t.Fatalf("amountCents = %v, want 5000", out["amountCents"])
	}
	if m.lastChargeAmount != 5000 {
		t.Fatalf("charged amount = %d, want 5000", m.lastChargeAmount)
	}
}

// The org's own account is subscribed by its admins; a plain member is refused. A
// member naming another member subscribes only their own account.
func TestOnlyAnOrgAdminSubscribesTheOrgAccount(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := moneyOrg("hanzo")
	member := map[string]string{"X-User-IsOrgAdmin": "false", "X-User-Id": "alice"}
	resp := invokeSubscribeCard(org, ctx, `{"planId":"max-20x"}`, member)
	if resp.StatusCode < 400 {
		t.Fatalf("a member subscribed with no card: %d %s", resp.StatusCode, bodyOf(resp))
	}
	resp = invokeSubscribeCard(org, ctx, `{"planId":"max-20x","userId":"hanzo/bob","sourceId":"credits"}`, member)
	if body := bodyOf(resp); strings.Contains(body, "hanzo/bob") {
		t.Fatalf("a member acted on another member's account: %d %s", resp.StatusCode, body)
	}
	db := datastore.New(org.Namespaced(ctx))
	for _, who := range []string{"hanzo/bob", "hanzo"} {
		if subs, _ := subscription.Query(db).Filter("UserId=", who).Count(); subs != 0 {
			t.Fatalf("a member opened %d subscription(s) on %s", subs, who)
		}
	}
	pooled := moneyOrg("acme")
	resp = invokeSubscribeCard(pooled, ctx, `{"planId":"team","quantity":2}`, map[string]string{"X-User-IsOrgAdmin": "false", "X-User-Id": "carol"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("a plain member of a pooled org subscribing its account: %d %s, want 403", resp.StatusCode, bodyOf(resp))
	}
}

// The org account is subscribed by a SuperAdmin or a service as well as the org's
// admins; X-User-IsAdmin alone does not make the caller an ORG admin.
func TestTheOrgAccountIsSubscribedByPrivilegedCallers(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("lux")
	for name, seed := range map[string]func(*zip.Ctx){
		"superadmin": func(c *zip.Ctx) {
			c.Locals("iam_authenticated", true)
			c.Locals("iam_claims", &auth.IAMClaims{Owner: "admin", IsAdmin: true})
		},
		"service": func(c *zip.Ctx) { c.Locals("permissions", bit.Field(permission.Admin|permission.Live)) },
	} {
		req := httptest.NewRequest(http.MethodPost, "/v1/billing/subscribe/card", bytes.NewBufferString(`{"sourceId":"credits","planId":"team","quantity":2}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-IsOrgAdmin", "false")
		resp := driveSeeded(func(c *zip.Ctx) {
			c.Locals("organization", org)
			c.SetContext(ctx)
			seed(c)
		}, "/v1/billing/subscribe/card", req, SubscribeWithCard)
		if resp.StatusCode == http.StatusForbidden {
			t.Errorf("%s subscribing the org account: 403 %s", name, bodyOf(resp))
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-IsAdmin", "true")
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Raw(zip.MethodAll, "/", func(c *zip.Ctx) error {
		if iammiddleware.ActsAsOrgAdmin(c) {
			t.Error("X-User-IsAdmin made the caller an org admin")
		}
		return c.JSON(200, nil)
	})
	if _, err := app.Test(req); err != nil {
		t.Fatal(err)
	}
}

// A plain member names ANOTHER member as the subject and pays with "balance": the
// other member's money must not buy a plan they did not ask for.
func TestAMemberCannotSpendAnotherMembersBalance(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := moneyOrg("acme-members")
	deposit(t, datastore.New(org.Namespaced(ctx)), "acme-members/bob", 100000)
	before := walletOf(t, ctx, org, "acme-members/bob")

	alice := map[string]string{"X-User-IsOrgAdmin": "false", "X-User-Id": "acme-members/alice"}
	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"balance","planId":"team","quantity":2,"userId":"acme-members/bob"}`, alice)
	body := bodyOf(resp)

	subs := parentSub(t, datastore.New(org.Namespaced(ctx)), "acme-members/bob", "team")
	after := walletOf(t, ctx, org, "acme-members/bob")
	if resp.StatusCode == http.StatusCreated || subs != nil {
		t.Fatalf("alice opened a subscription for bob paid from bob's balance: status=%d balance %d -> %d (%s)",
			resp.StatusCode, before, after, body)
	}
}

// The same subject choice with sourceId "credits": a member must not spend
// another member's credit on a plan for them.
func TestAMemberCannotBurnAnotherMembersCredits(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := moneyOrg("acme-credits")
	db := datastore.New(org.Namespaced(ctx))
	g := creditgrant.New(db)
	g.UserId, g.Name = "acme-credits/bob", "starter"
	g.AmountCents, g.RemainingCents, g.Currency, g.Priority = 10000, 10000, currency.USD, 1
	if err := g.Create(); err != nil {
		t.Fatal(err)
	}

	alice := map[string]string{"X-User-IsOrgAdmin": "false", "X-User-Id": "acme-credits/alice"}
	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"credits","planId":"team","quantity":2,"userId":"acme-credits/bob"}`, alice)
	body := bodyOf(resp)
	grants, _ := getActiveGrants(db, "acme-credits/bob")
	var left int64
	for _, x := range grants {
		left += x.RemainingCents
	}
	if resp.StatusCode == http.StatusCreated || left != 10000 {
		t.Fatalf("alice spent bob's credit on a plan for bob: status=%d, bob's credit 10000 -> %d (%s)", resp.StatusCode, left, body)
	}
}

// A member names themselves and sends no card: no subscription, no tier.
func TestACustomerGetsNoFreeSubscription(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := moneyOrg("hanzo")
	member := map[string]string{"X-User-IsOrgAdmin": "false", "X-User-Id": "hanzo/carol"}
	resp := invokeSubscribeCard(org, ctx, `{"planId":"team","quantity":2,"userId":"hanzo/carol"}`, member)
	if resp.StatusCode == http.StatusCreated {
		t.Fatalf("a hanzo customer subscribed with no card and no balance: %s", bodyOf(resp))
	}
	if got, _ := TierOf(ctx, org, "hanzo/carol"); got != tier.Free {
		t.Fatalf("a hanzo customer reads as %s", got)
	}
}

// A plain member of a pooled org carries no billing claim and pays from the org's
// account; a signup-org customer pays from their own. Either way the tier is what
// that account's own subscriptions confer — Free here, with none recorded.
func TestPooledMembersPayFromTheOrgAccount(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	for _, tc := range []struct {
		org, user, claim, subject string
	}{
		{"lux", "dev", "", "lux"},
		{"zoo", "dev", "", "zoo"},
		{"lux", "lux/owner", "org:lux", "lux"},
		{"hanzo", "hanzo/carol", "person:hanzo/carol", "hanzo/carol"},
		{"hanzo", "carol", "", "hanzo/carol"},
	} {
		got := subjectFor(t, tc.org, tc.user, tc.claim)
		if got != tc.subject {
			t.Errorf("%s in %s (claim %q) pays from %q, want %q", tc.user, tc.org, tc.claim, got, tc.subject)
			continue
		}
		name, err := TierOf(ctx, moneyOrg(tc.org), got)
		if err != nil {
			t.Fatal(err)
		}
		if name != tier.Free {
			t.Errorf("%s pays from %s at tier %s, want free", tc.user, got, name)
		}
	}
}
