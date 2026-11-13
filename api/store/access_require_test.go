package store

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	billing "github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	storemodel "github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// callRequireAccess drives the real RequireAccess gate for org `ns`, with the
// given X-Store-Id, in front of a sentinel that returns 200. It returns the
// final status so the test can assert 402 (blocked) vs 200 (allowed).
func callRequireAccess(t *testing.T, ns, storeID string) int {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	seed := func(c *zip.Ctx) error {
		org := &organization.Organization{}
		org.Name = ns
		c.Locals("organization", org)
		c.SetContext(nscontext.WithNamespace(context.Background(), ns))
		return c.Next()
	}
	sentinel := func(c *zip.Ctx) error { return c.String(http.StatusOK, "ok") }
	app.Raw(http.MethodGet, "/protected", seed, RequireAccess, sentinel)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if storeID != "" {
		req.Header.Set("X-Store-Id", storeID)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

func seedStore(t *testing.T, ns string) *storemodel.Store {
	t.Helper()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))
	s := storemodel.New(db)
	s.Name = "Acme Shop"
	if err := s.Create(); err != nil {
		t.Fatalf("create store: %v", err)
	}
	return s
}

func seedProSub(t *testing.T, ns, storeID string, status subscription.Status, periodEnd, trialEnd time.Time) {
	t.Helper()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))
	sub := subscription.New(db)
	sub.StoreId = storeID
	sub.PlanId = "go"
	sub.Plan.Slug, sub.Plan.Category, sub.Plan.Price = "go", "personal", 800
	sub.ProviderType = "square"
	sub.Status = status
	sub.PeriodStart = time.Now().Add(-time.Hour)
	sub.PeriodEnd = periodEnd
	sub.TrialEnd = trialEnd
	if !trialEnd.IsZero() {
		sub.TrialStart = time.Now().Add(-time.Hour)
	}
	if err := sub.Create(); err != nil {
		t.Fatalf("create sub: %v", err)
	}
}

// TestRequireAccess_NoStore402 proves the gate refuses (402) when no store can be
// resolved for the org.
func TestRequireAccess_NoStore402(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	if _, _, err := billing.SeedPlans(tc); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if code := callRequireAccess(t, "acme", "no-such-store"); code != http.StatusPaymentRequired {
		t.Fatalf("no-store access = %d, want 402", code)
	}
}

// TestRequireAccess_UnpaidStore402 proves a resolvable store with no current plan
// subscription is blocked with 402 (payment_required) — the authoritative paywall.
func TestRequireAccess_UnpaidStore402(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	if _, _, err := billing.SeedPlans(tc); err != nil {
		t.Fatalf("seed: %v", err)
	}

	s := seedStore(t, "acme")
	if code := callRequireAccess(t, "acme", s.Id()); code != http.StatusPaymentRequired {
		t.Fatalf("unpaid store access = %d, want 402", code)
	}
}

// TestRequireAccess_ActivePlanAllows proves a store with a current active pro
// subscription passes the gate through to the protected handler (200).
func TestRequireAccess_ActivePlanAllows(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	if _, _, err := billing.SeedPlans(tc); err != nil {
		t.Fatalf("seed: %v", err)
	}

	s := seedStore(t, "acme")
	seedProSub(t, "acme", s.Id(), subscription.Active, time.Now().Add(time.Hour), time.Time{})

	if code := callRequireAccess(t, "acme", s.Id()); code != http.StatusOK {
		t.Fatalf("active-plan access = %d, want 200 (gate lets it through)", code)
	}
}

// TestRequireAccess_ExpiredActiveBlocked proves an active subscription whose
// period has already ended does NOT unlock — the gate re-checks period end and
// returns 402.
func TestRequireAccess_ExpiredActiveBlocked(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	if _, _, err := billing.SeedPlans(tc); err != nil {
		t.Fatalf("seed: %v", err)
	}

	s := seedStore(t, "acme")
	seedProSub(t, "acme", s.Id(), subscription.Active, time.Now().Add(-time.Hour), time.Time{})

	if code := callRequireAccess(t, "acme", s.Id()); code != http.StatusPaymentRequired {
		t.Fatalf("expired-active access = %d, want 402", code)
	}
}

// TestRequireAccess_TrialingAllows proves a trial opened before commerce stopped
// selling them unlocks the gate until its end.
func TestRequireAccess_TrialingAllows(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	if _, _, err := billing.SeedPlans(tc); err != nil {
		t.Fatalf("seed: %v", err)
	}

	s := seedStore(t, "acme")
	seedProSub(t, "acme", s.Id(), subscription.Trialing, time.Time{}, time.Now().Add(48*time.Hour))

	if code := callRequireAccess(t, "acme", s.Id()); code != http.StatusOK {
		t.Fatalf("trialing access = %d, want 200", code)
	}
}

// TestRequireAccess_StoreBound proves the store-bound rule at the gate: a plan on
// store A does not unlock store B.
func TestRequireAccess_StoreBound(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	if _, _, err := billing.SeedPlans(tc); err != nil {
		t.Fatalf("seed: %v", err)
	}

	a := seedStore(t, "acme")
	b := seedStore(t, "acme")
	seedProSub(t, "acme", a.Id(), subscription.Active, time.Now().Add(time.Hour), time.Time{})

	if code := callRequireAccess(t, "acme", a.Id()); code != http.StatusOK {
		t.Fatalf("store A (subscribed) = %d, want 200", code)
	}
	if code := callRequireAccess(t, "acme", b.Id()); code != http.StatusPaymentRequired {
		t.Fatalf("store B (no plan) = %d, want 402 (A's plan must not unlock B)", code)
	}
}

// Store access end to end on a real store, with no trial anywhere: the
// @hanzo/plans catalog is seeded into the test database as a boot does; a new
// store with no plan is refused (there is no trial to start, and a trial credit
// opens nothing); a store whose plan its card pays for — a rung on sale, or a
// retired rung a holder still pays for — passes the gate.
func TestE2E_StoreAccessOpensOnAPaidPlanAndNoTrial(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	if _, _, err := billing.SeedPlans(tc); err != nil {
		t.Fatalf("seed: %v", err)
	}

	fresh := seedStore(t, "acme")
	if code := callRequireAccess(t, "acme", fresh.Id()); code != http.StatusPaymentRequired {
		t.Fatalf("a new store with no plan: %d, want 402", code)
	}
	db := datastore.New(nscontext.WithNamespace(context.Background(), "acme"))
	credit := transaction.New(db)
	credit.Type, credit.DestinationId, credit.DestinationKind = transaction.Deposit, "acme", "iam-user"
	credit.Currency, credit.Amount, credit.Tags = "usd", currency.Cents(2000), "trial-credit:"+fresh.Id()
	credit.ExpiresAt = time.Now().Add(7 * 24 * time.Hour)
	if err := credit.Create(); err != nil {
		t.Fatalf("trial credit: %v", err)
	}
	if code := callRequireAccess(t, "acme", fresh.Id()); code != http.StatusPaymentRequired {
		t.Fatalf("a store with a trial credit and no plan: %d, want 402", code)
	}

	for _, slug := range []string{"go", "ultimate", "dev"} {
		s := seedStore(t, "acme")
		p := billing.LookupStaticPlan(slug)
		if p == nil {
			t.Fatalf("the catalog has no %s", slug)
		}
		sub := subscription.New(db)
		sub.UserId, sub.StoreId, sub.PlanId = "acme", s.Id(), slug
		sub.Plan.Slug, sub.Plan.Category, sub.Plan.Price = slug, p.Category, currency.Cents(p.PriceMonth)
		sub.ProviderType, sub.Status = "square", subscription.Active
		sub.PeriodStart, sub.PeriodEnd = time.Now().Add(-time.Hour), time.Now().AddDate(0, 1, 0)
		if err := sub.Create(); err != nil {
			t.Fatalf("%s subscription: %v", slug, err)
		}
		if code := callRequireAccess(t, "acme", s.Id()); code != http.StatusOK {
			t.Fatalf("a store on %s paid by card: %d, want 200", slug, code)
		}
	}
}
