package metrics

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
)

// testApp backs the synthetic request contexts the gate tests run against.
var testApp = zip.New(zip.Config{DisableStartupMessage: true})

// gateCtx builds a zip test context carrying optional IAM claims and/or a
// permissions bit-field, mirroring what the auth middleware would have set.
func gateCtx(claims *auth.IAMClaims, perms *bit.Field) *zip.Ctx {
	c := testApp.TestCtx(http.MethodGet, "/v1/commerce/metrics/saas")
	if claims != nil {
		c.Locals("iam_claims", claims)
	}
	if perms != nil {
		c.Locals("permissions", *perms)
	}
	return c
}

func iamUser(subject, owner string, isAdmin bool) *auth.IAMClaims {
	cl := &auth.IAMClaims{Owner: owner, IsAdmin: isAdmin}
	cl.Subject = subject
	return cl
}

func adminPerms() *bit.Field {
	f := bit.Field(permission.Admin | permission.Live)
	return &f
}

// TestGetSaaS_RefusesNonPlatformAdmin is the CRITICAL authz proof: the SaaS
// god-view is cross-tenant business data, so anyone who is not a PLATFORM
// SuperAdmin (or the trusted internal service token) must get 403 with NO data
// leaked — before the handler ever touches the datastore. The headline attack is
// the org-level admin: the gateway mints permission.Admin from a tenant's own
// IsAdmin, so the Admin bit is present, but the caller carries an IAM Subject and
// its home org is NOT "admin" — it is not a SuperAdmin and must be refused.
func TestGetSaaS_RefusesNonPlatformAdmin(t *testing.T) {
	cases := []struct {
		name   string
		claims *auth.IAMClaims
		perms  *bit.Field
	}{
		{"org admin tenant (Admin bit + IAM subject)", iamUser("u2", "acme", true), adminPerms()},
		{"plain tenant (no admin)", iamUser("u3", "acme", false), nil},
		{"iam user, Admin claim but no perms bit", iamUser("u4", "acme", true), nil},
		{"no auth context at all", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := gateCtx(tc.claims, tc.perms)
			_ = GetSaaS(c)
			status := c.Fiber().Response().StatusCode()
			body := string(c.Fiber().Response().Body())
			if status != 403 {
				t.Fatalf("%s got %d, want 403 (must never read cross-org SaaS metrics; body=%s)", tc.name, status, body)
			}
			if strings.Contains(body, "mrrCents") || strings.Contains(body, "\"revenue\"") {
				t.Fatalf("403 body leaked metrics: %s", body)
			}
		})
	}
}

// TestGate_AdmitsPlatformPrincipals proves the accept side of the shared gate: a
// SuperAdmin (home org == the built-in "admin" org) and the trusted M2M service
// token (Admin bit + no IAM Subject) are admitted; org admins are not.
func TestGate_AdmitsPlatformPrincipals(t *testing.T) {
	cases := []struct {
		name   string
		claims *auth.IAMClaims
		perms  *bit.Field
		want   bool
	}{
		{"superadmin (admin org)", iamUser("u1", "admin", false), adminPerms(), true},
		{"service token (Admin bit, no IAM user)", &auth.IAMClaims{}, adminPerms(), true},
		{"service token (nil claims, Admin bit)", nil, adminPerms(), true},
		{"org admin tenant — REJECT", iamUser("u2", "acme", true), adminPerms(), false},
		{"plain tenant — REJECT", iamUser("u3", "acme", false), nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := gateCtx(tc.claims, tc.perms)
			if got := middleware.MayReadPlatform(c); got != tc.want {
				t.Fatalf("MayReadPlatform = %v, want %v", got, tc.want)
			}
		})
	}
}

// mkSub builds an in-memory subscription for the pure-fold aggregation test (no
// datastore). CreatedAt is on the embedded mixin base model; canceled subs also set
// the Canceled flag + CanceledAt.
func mkSub(slug string, priceCents int64, interval types.Interval, status subscription.Status, qty int, created, canceled time.Time, provider string) *subscription.Subscription {
	s := &subscription.Subscription{PlanId: slug, Status: status, Quantity: qty, ProviderType: provider}
	s.Plan = plan.Plan{Slug: slug, Name: slug, Price: currency.Cents(priceCents), Interval: interval}
	s.CreatedAt = created
	if !canceled.IsZero() {
		s.Canceled = true
		s.CanceledAt = canceled
	}
	return s
}

func mkTxn(cents int64, created time.Time, model string) *transaction.Transaction {
	tr := &transaction.Transaction{Amount: currency.Cents(cents), Test: false, Tags: "api-usage"}
	tr.CreatedAt = created
	if model != "" {
		tr.Metadata = types.Map{"model": model}
	} else {
		tr.Metadata = types.Map{}
	}
	return tr
}

// TestRollupAggregation exercises the money math end to end (pure fold, no
// datastore): MRR/ARR normalization, active/trial/paying counts, new/churn inside
// the window, bundle-child exclusion, category bucketing, billed usage totals, and
// the top-customers projection. Every figure is asserted against a hand-computed
// expectation so a regression in the fold is caught.
func TestRollupAggregation(t *testing.T) {
	now := time.Now().UTC()
	a := newAcc(now, windowStart(now, "30d"))

	acmeMonthly := mkSub("plan-a", 2000, types.Monthly, subscription.Active, 1, now.AddDate(0, 0, -10), time.Time{}, "internal")
	acmeAnnual := mkSub("plan-b", 12000, types.Yearly, subscription.Active, 1, now.AddDate(0, 0, -100), time.Time{}, "internal")
	acmeBundle := mkSub("world-x", 0, types.Monthly, subscription.Active, 1, now.AddDate(0, 0, -10), time.Time{}, "bundle") // skipped
	betaTrial := mkSub("plan-a", 2000, types.Monthly, subscription.Trialing, 1, now.AddDate(0, 0, -2), time.Time{}, "internal")
	gammaCancel := mkSub("plan-c", 5000, types.Monthly, subscription.Canceled, 1, now.AddDate(0, 0, -40), now.AddDate(0, 0, -5), "internal")

	foldSubs(a, "acme", []*subscription.Subscription{acmeMonthly, acmeAnnual, acmeBundle}, false)
	foldSubs(a, "beta", []*subscription.Subscription{betaTrial}, false)
	foldSubs(a, "gamma", []*subscription.Subscription{gammaCancel}, false)

	foldTxns(a, "acme", []*transaction.Transaction{
		mkTxn(100, now.AddDate(0, 0, -1), "gpt-4o"),
		mkTxn(100, now.AddDate(0, 0, -3), "gpt-4o"),
	}, false)
	foldTxns(a, "beta", []*transaction.Transaction{
		mkTxn(50, now.AddDate(0, 0, -1), "claude-3-5"),
		mkTxn(30, now.AddDate(0, 0, -1), ""),         // untagged
		mkTxn(999, now.AddDate(0, 0, -60), "gpt-4o"), // outside 30d window → excluded
	}, false)
	a.orgSeen["acme"], a.orgSeen["beta"], a.orgSeen["gamma"] = true, true, true

	s := a.snapshot(options{window: "30d", limit: 20})

	// Revenue: MRR = 2000 (acme monthly) + 1000 (acme annual /12); trial + canceled excluded.
	if s.Revenue.MRRCents != 3000 {
		t.Errorf("MRR = %d, want 3000", s.Revenue.MRRCents)
	}
	if s.Revenue.ARRCents != 36000 {
		t.Errorf("ARR = %d, want 36000", s.Revenue.ARRCents)
	}
	if s.Revenue.ActiveSubscriptions != 2 {
		t.Errorf("active subs = %d, want 2 (bundle child excluded)", s.Revenue.ActiveSubscriptions)
	}
	if s.Revenue.Trials != 1 {
		t.Errorf("trials = %d, want 1", s.Revenue.Trials)
	}
	if s.Revenue.PayingCustomers != 1 {
		t.Errorf("paying customers = %d, want 1 (only acme)", s.Revenue.PayingCustomers)
	}
	// New: acme monthly (created -10d, active, +2000) and beta trial (created -2d, +2000).
	if s.Subs.New != 2 || s.Revenue.NewMRRCents != 4000 {
		t.Errorf("new = %d newMRR = %d, want 2 / 4000", s.Subs.New, s.Revenue.NewMRRCents)
	}
	// Churn: gamma canceled -5d, would-be 5000.
	if s.Subs.Canceled != 1 || s.Revenue.ChurnedMRRCents != 5000 {
		t.Errorf("canceled = %d churnedMRR = %d, want 1 / 5000", s.Subs.Canceled, s.Revenue.ChurnedMRRCents)
	}
	if s.Revenue.NetNewMRRCents != -1000 {
		t.Errorf("netNewMRR = %d, want -1000", s.Revenue.NetNewMRRCents)
	}
	// Usage: window billed = 100+100+50+30 = 280; requests = 4; one untagged.
	if s.Usage.WindowUsageCents != 280 || s.Usage.Requests != 4 {
		t.Errorf("usage = %d / %d req, want 280 / 4", s.Usage.WindowUsageCents, s.Usage.Requests)
	}
	if !s.Usage.Instrumented || s.Usage.UntaggedRequests != 1 {
		t.Errorf("instrumented=%v untagged=%d, want true / 1", s.Usage.Instrumented, s.Usage.UntaggedRequests)
	}
	// Events: 2 created + 1 canceled, most-recent first (beta -2d, gamma -5d, acme -10d).
	if len(s.Subs.Recent) != 3 {
		t.Fatalf("events = %d, want 3", len(s.Subs.Recent))
	}
	if s.Subs.Recent[0].Org != "beta" || s.Subs.Recent[0].Type != "created" {
		t.Errorf("most recent event = %+v, want beta/created", s.Subs.Recent[0])
	}
	// Upgrades/downgrades are not instrumented → null (honest, never fabricated).
	if s.Subs.Upgrades != nil || s.Subs.Downgrades != nil {
		t.Errorf("upgrades/downgrades must be null (not instrumented), got %v/%v", s.Subs.Upgrades, s.Subs.Downgrades)
	}
	// Customers: acme (mrr 3000 + usage 200) ranks above beta (mrr 0 + usage 80);
	// gamma (canceled, no active mrr, no usage) does not appear.
	if len(s.Customers) != 2 {
		t.Fatalf("customers = %d, want 2 (gamma excluded)", len(s.Customers))
	}
	if s.Customers[0].Org != "acme" || s.Customers[0].MRRCents != 3000 || s.Customers[0].UsageCents != 200 {
		t.Errorf("top customer = %+v, want acme mrr 3000 usage 200", s.Customers[0])
	}
	if s.Customers[1].Org != "beta" || s.Customers[1].Status != string(subscription.Trialing) || s.Customers[1].UsageCents != 80 {
		t.Errorf("second customer = %+v, want beta trialing usage 80", s.Customers[1])
	}
	if s.Orgs != 3 {
		t.Errorf("orgs = %d, want 3", s.Orgs)
	}
}

// TestPlanMeta_FromCatalog proves category/name resolution reads the embedded
// @hanzo/plans catalog (the Stripe-seed SOT) for a real slug and falls back
// honestly for an unknown one.
func TestPlanMeta_FromCatalog(t *testing.T) {
	plans := billing.StaticPlans()
	if len(plans) == 0 {
		t.Fatal("embedded plan catalog is empty — commerce cannot map plan categories")
	}
	p := plans[0]
	cat, name := planMeta(p.Slug, "")
	if p.Category != "" && cat != p.Category {
		t.Errorf("planMeta(%q) category = %q, want %q", p.Slug, cat, p.Category)
	}
	if name == "" {
		t.Errorf("planMeta(%q) name is empty", p.Slug)
	}
	if cat2, name2 := planMeta("__no_such_plan__", "Embedded Name"); cat2 != "uncategorized" || name2 != "Embedded Name" {
		t.Errorf("unknown slug = (%q,%q), want (uncategorized, Embedded Name)", cat2, name2)
	}
}

// TestWindowBounds checks window parsing + inWindow, including the "all" (zero
// start) and skew-tolerance behaviour.
func TestWindowBounds(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	if got := windowStart(now, "7d"); !got.Equal(now.AddDate(0, 0, -7)) {
		t.Errorf("7d start = %v", got)
	}
	if got := windowStart(now, "mtd"); got.Day() != 1 || got.Month() != 7 {
		t.Errorf("mtd start = %v, want first of month", got)
	}
	if got := windowStart(now, "all"); !got.IsZero() {
		t.Errorf("all start = %v, want zero", got)
	}
	start := windowStart(now, "30d")
	if !inWindow(now.AddDate(0, 0, -5), start, now) {
		t.Error("-5d should be in a 30d window")
	}
	if inWindow(now.AddDate(0, 0, -31), start, now) {
		t.Error("-31d should be outside a 30d window")
	}
	if inWindow(time.Time{}, start, now) {
		t.Error("zero time is never in-window")
	}
}

// TestParseLimit clamps the top-N cap.
func TestParseLimit(t *testing.T) {
	cases := map[string]int{"": defaultLimit, "0": defaultLimit, "-5": defaultLimit, "abc": defaultLimit, "10": 10, "500": 200}
	for in, want := range cases {
		if got := parseLimit(in); got != want {
			t.Errorf("parseLimit(%q) = %d, want %d", in, got, want)
		}
	}
}
