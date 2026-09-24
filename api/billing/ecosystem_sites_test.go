package billing

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
)

// Every site the ecosystem rule decides, asked about a person in an ecosystem org
// and about the org's own account: the person is a customer, the account is not.

func TestTheBalanceFloorIsTheEcosystemAccountsAlone(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("hanzo")
	read := func(user string) int64 {
		req := httptest.NewRequest(http.MethodGet, "/v1/billing/balance?user="+user+"&currency=usd", nil)
		resp := driveSeeded(func(c *zip.Ctx) {
			c.Locals("organization", org)
			c.SetContext(ctx)
		}, "/v1/billing/balance", req, GetBalance)
		var out struct {
			Available int64 `json:"available"`
		}
		raw, _ := io.ReadAll(resp.Body)
		_ = json.Unmarshal(raw, &out)
		return out.Available
	}
	if got := read("hanzo/alice"); got != 0 {
		t.Errorf("a person's balance reads %d, want 0 (no ecosystem floor)", got)
	}
	if got := read("hanzo"); got < DefaultEcosystemCreditCents {
		t.Errorf("the org account's balance reads %d, want at least the ecosystem floor %d", got, DefaultEcosystemCreditCents)
	}
}

func TestTheCreditFloorsAreTheEcosystemAccountsAlone(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("hanzo")
	for _, c := range []struct {
		user  string
		floor bool
	}{{"hanzo/alice", false}, {"hanzo", true}} {
		b, err := ReadCreditBalance(ctx, org, c.user)
		if err != nil {
			t.Fatal(err)
		}
		var usd int64
		for _, x := range b.Balances {
			if x.Currency == "usd" {
				usd = x.Available
			}
		}
		if (usd >= DefaultEcosystemCreditCents) != c.floor {
			t.Errorf("ReadCreditBalance(%s) usd = %d, floor wanted %v", c.user, usd, c.floor)
		}
		bd, err := ReadCreditBreakdown(ctx, org, c.user)
		if err != nil {
			t.Fatal(err)
		}
		if (bd.Total.Cents >= DefaultEcosystemCreditCents) != c.floor {
			t.Errorf("ReadCreditBreakdown(%s) total = %d, floor wanted %v", c.user, bd.Total.Cents, c.floor)
		}
	}
}

// POST /v1/billing/subscriptions: a person in an ecosystem org is not a mint
// principal by prefix; the platform's own application still is.
func TestAPaidSubscriptionForAPersonInAnEcosystemOrgNeedsAMinter(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("hanzo")
	orgAdmin := func(c *zip.Ctx) {
		c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
		c.Locals("iam_authenticated", true)
		c.Locals("iam_claims", &auth.IAMClaims{Owner: "hanzo", IsAdmin: true})
	}
	w := invokeSub(org, ctx, orgAdmin, CreateBillingSubscription, `{"userId":"hanzo/alice","planId":"max-5x"}`)
	if w.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(w.Body)
		t.Fatalf("a paid plan for a person in hanzo without a minter: %d %s, want 403", w.StatusCode, b)
	}
	w = invokeSub(org, ctx, c1MintPrincipal, CreateBillingSubscription, `{"userId":"hanzo","planId":"max-5x"}`)
	if w.StatusCode == http.StatusForbidden {
		b, _ := io.ReadAll(w.Body)
		t.Fatalf("the platform's application subscribing the org account: 403 %s", b)
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
			b, _ := io.ReadAll(resp.Body)
			t.Errorf("%s subscribing the org account: 403 %s", name, b)
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
