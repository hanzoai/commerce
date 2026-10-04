package billing

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/billing/tier"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/creditgrant"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
)

// Our own orgs. Each resolves exactly like a customer's: a plan only from a real
// recorded subscription, a balance only from money on the ledger.
var ourOrgs = []string{"hanzo", "admin", "lux", "zoo", "adnexus", "bootnode", "osage", "pars"}

func balanceAvailable(t *testing.T, ctx context.Context, org *organization.Organization, user string) int64 {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/billing/balance?user="+user+"&currency=usd", nil)
	resp := driveSeeded(func(c *zip.Ctx) {
		c.Locals("organization", org)
		c.SetContext(ctx)
	}, "/v1/billing/balance", req, GetBalance)
	var out struct {
		Available int64 `json:"available"`
	}
	raw, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("balance %s: %d %s", user, resp.StatusCode, raw)
	}
	return out.Available
}

// With no subscription and no money, every one of our orgs is Free with nothing
// to spend — the org account and a person inside it alike.
func TestOurOrgsResolveLikeCustomers(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	for _, name := range ourOrgs {
		org := moneyOrg(name)
		for _, subject := range []string{name, name + "/alice"} {
			if got, err := TierOf(ctx, org, subject); err != nil || got != tier.Free {
				t.Errorf("TierOf(%s) = %s, %v; want free", subject, got, err)
			}
			v, err := ReadTier(ctx, org, subject)
			if err != nil {
				t.Fatalf("ReadTier(%s): %v", subject, err)
			}
			if v.Tier.Name != tier.Free || v.Plan != "" || v.Subscription != "" {
				t.Errorf("ReadTier(%s) = tier %s plan %q row %q; want free with no plan", subject, v.Tier.Name, v.Plan, v.Subscription)
			}
			if v.Balance.EffectiveAvailable != 0 || v.Balance.CreditsRemaining != 0 || v.Balance.PrepaidAvailable != 0 {
				t.Errorf("ReadTier(%s) balance %+v; want zero — there is no floor", subject, v.Balance)
			}
			for _, m := range v.Tier.AllowedModels {
				if m == "*" {
					t.Errorf("ReadTier(%s) allows every model", subject)
				}
			}
			if got := balanceAvailable(t, ctx, org, subject); got != 0 {
				t.Errorf("GET /balance %s = %d; want 0", subject, got)
			}
			cb, err := ReadCreditBalance(ctx, org, subject)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range cb.Balances {
				if e.Available != 0 {
					t.Errorf("ReadCreditBalance(%s) %s = %d; want 0", subject, e.Currency, e.Available)
				}
			}
			bd, err := ReadCreditBreakdown(ctx, org, subject)
			if err != nil {
				t.Fatal(err)
			}
			if bd.Total.Cents != 0 || len(bd.Breakdown) != 0 {
				t.Errorf("ReadCreditBreakdown(%s) = %d over %v; want nothing", subject, bd.Total.Cents, bd.Breakdown)
			}
		}
	}
}

// A real recorded subscription is the one way any of them gets a plan.
func TestOurOrgsGetAPlanOnlyFromARecordedSubscription(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("hanzo")
	row := seedRow(t, datastore.New(org.Namespaced(ctx)), "hanzo", "dev", "square", time.Now().AddDate(0, 0, -3))
	v, err := ReadTier(ctx, org, "hanzo")
	if err != nil {
		t.Fatal(err)
	}
	if v.Tier.Name != tier.Pro || v.Plan != "dev" || v.Subscription != row.Id() {
		t.Fatalf("hanzo with a paid dev row: tier %s plan %q row %q; want pro from %s", v.Tier.Name, v.Plan, v.Subscription, row.Id())
	}
}

// A subscription for our orgs takes payment like anyone's: no card and no
// source is refused, credits with nothing behind them are declined, and money
// on the ledger pays.
func TestOurOrgsPayToSubscribe(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	zoo := moneyOrg("zoo")
	resp := invokeSubscribeCard(zoo, ctx, `{"planId":"team","quantity":2}`, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("zoo with no card: %d %s; want 400", resp.StatusCode, bodyOf(resp))
	}
	if s := parentSub(t, datastore.New(zoo.Namespaced(ctx)), "zoo", "team"); s != nil {
		t.Fatalf("zoo opened %s with no payment", s.Id())
	}

	lux := moneyOrg("lux")
	resp = invokeSubscribeCard(lux, ctx, `{"sourceId":"credits","planId":"team","quantity":2}`, nil)
	if resp.StatusCode < 400 {
		t.Fatalf("lux paying from an empty wallet: %d %s; want a refusal", resp.StatusCode, bodyOf(resp))
	}
	luxDB := datastore.New(lux.Namespaced(ctx))
	if s := parentSub(t, luxDB, "lux", "team"); s != nil {
		t.Fatalf("lux opened %s from an empty wallet", s.Id())
	}

	deposit(t, luxDB, "lux", 5000)
	resp = invokeSubscribeCard(lux, ctx, `{"sourceId":"balance","planId":"team","quantity":2}`, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("lux paying 5000 from a funded wallet: %d %s; want 201", resp.StatusCode, bodyOf(resp))
	}
	if got := walletOf(t, ctx, lux, "lux"); got != 0 {
		t.Fatalf("lux wallet after paying 5000 of 5000 = %d; want 0", got)
	}

	// The org's own admin is not a minter, so a paid plan for the org account
	// through the no-payment create path is refused like any tenant's.
	hanzo := moneyOrg("hanzo")
	orgAdmin := func(c *zip.Ctx) {
		c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
		c.Locals("iam_authenticated", true)
		c.Locals("iam_claims", &auth.IAMClaims{Owner: "hanzo", IsAdmin: true})
	}
	w := invokeSub(hanzo, ctx, orgAdmin, CreateBillingSubscription, `{"userId":"hanzo","planId":"max-5x"}`)
	if w.StatusCode != http.StatusForbidden {
		t.Fatalf("hanzo's admin opening max-5x for hanzo unpaid: %d %s; want 403", w.StatusCode, bodyOf(w))
	}
	if n, _ := subscription.Query(datastore.New(hanzo.Namespaced(ctx))).Filter("UserId=", "hanzo").Count(); n != 0 {
		t.Fatalf("hanzo holds %d subscription(s) nobody paid for", n)
	}
}

// The auto-recharge sweep charges cards that opted in. It mints nothing: no org
// is topped up from thin air.
func TestNoGrantIsMintedForOurOrgs(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	root := datastore.New(ctx)
	for _, name := range ourOrgs {
		o := organization.New(root)
		o.Name = name
		o.Live = true
		if err := o.Create(); err != nil {
			t.Fatalf("seed org %s: %v", name, err)
		}
	}
	if _, err := RunAutoRecharge(ctx, nil, nil); err != nil {
		t.Fatalf("RunAutoRecharge: %v", err)
	}
	for _, name := range ourOrgs {
		db := datastore.New(moneyOrg(name).Namespaced(ctx))
		if n, _ := creditgrant.Query(db).Count(); n != 0 {
			t.Errorf("%s holds %d grant(s) after the sweep; want none", name, n)
		}
	}
}

// The tier is the subject's own. A SuperAdmin naming one — by X-Tier or ?tier= —
// changes nothing.
func TestTierIgnoresAnOverrideFromASuperAdmin(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("tier-ovr")
	for _, h := range []zip.Handler{GetTier, TierCheck} {
		for _, target := range []string{"/t?user=tier-ovr/alice&tier=enterprise", "/t?user=tier-ovr/alice"} {
			req := httptest.NewRequest(http.MethodGet, target, nil)
			req.Header.Set("X-User-Owner", "admin") // SuperAdmin
			req.Header.Set("X-Tier", "enterprise")
			w := driveSeeded(tierSeed(org), "/t", req, h)
			var out map[string]any
			raw, _ := io.ReadAll(w.Body)
			_ = json.Unmarshal(raw, &out)
			if w.StatusCode != 200 || tierNameOf(out) != string(tier.Free) {
				t.Errorf("%s with a SuperAdmin's X-Tier: %d tier %q; want 200 free (%s)", target, w.StatusCode, tierNameOf(out), raw)
			}
		}
	}
}

// A person's wallet spends that person's grants. The org account's grants are
// the org account's: a member billing personally reaches none of them.
func TestAMemberWalletDoesNotSpendOrgGrants(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("acme-pool")
	db := datastore.New(org.Namespaced(ctx))
	orgGrant := grant(t, db, "acme-pool", 10000)

	if gs, err := getActiveGrants(db, "acme-pool/alice"); err != nil || len(gs) != 0 {
		t.Fatalf("alice sees %d grant(s), %v; want none of the org's", len(gs), err)
	}
	cb, err := ReadCreditBalance(ctx, org, "acme-pool/alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(cb.Balances) != 0 {
		t.Fatalf("alice's credit balance %+v; want empty", cb.Balances)
	}
	over, err := BurnCredits(db, "acme-pool/alice", 2500, "")
	if err != nil || over != 2500 {
		t.Fatalf("alice burning 2500: overage %d, %v; want all 2500 unpaid by the org", over, err)
	}
	back := creditgrant.New(db)
	if err := back.GetById(orgGrant.Id()); err != nil {
		t.Fatal(err)
	}
	if back.RemainingCents != 10000 {
		t.Fatalf("the org's grant holds %d after alice's burn; want 10000", back.RemainingCents)
	}
}
