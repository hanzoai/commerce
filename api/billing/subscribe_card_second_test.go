package billing

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/test/ae"
)

// ONE PAID SUBSCRIPTION PER SUBJECT.
//
// This endpoint only ever STARTED a subscription — charge, then createSubscription,
// with nothing looking for one the subject already had. The idempotency guard keys
// on (subject, store, plan), so a DIFFERENT plan replayed nothing: a paying Pro
// customer clicking Upgrade on Max was charged $99 in full, got a SECOND active
// subscription, and kept renewing the first.
//
// The exclusions matter as much as the refusal: a FREE row and a zero-payment
// internal row both collect nothing, so neither may block the customer who is
// finally paying. free → paid is the main funnel.

// liveSub seeds an Active subscription for subject on planSlug. providerType picks
// whether it is payment-backed ("square") or a zero-payment internal row.
func liveSub(t *testing.T, db *datastore.Datastore, subject, planSlug, providerType string) *subscription.Subscription {
	t.Helper()
	p, err := resolveSubscriptionPlan(db, planSlug)
	if err != nil {
		t.Fatalf("resolve plan %q: %v", planSlug, err)
	}
	s := subscription.New(db)
	s.UserId = subject
	s.PlanId = planSlug
	s.Plan = *p
	s.Quantity = 1
	s.Status = subscription.Active
	s.ProviderType = providerType
	s.PeriodStart = time.Now().Add(-24 * time.Hour)
	s.PeriodEnd = time.Now().Add(6 * 24 * time.Hour)
	if err := s.Create(); err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	return s
}

func liveSubs(t *testing.T, db *datastore.Datastore, subject string) []*subscription.Subscription {
	t.Helper()
	out := make([]*subscription.Subscription, 0)
	if _, err := subscription.Query(db).Filter("UserId=", subject).GetAll(&out); err != nil {
		t.Fatalf("query subscriptions: %v", err)
	}
	live := out[:0]
	for _, s := range out {
		if s.ProviderType != "bundle" && s.Status == subscription.Active {
			live = append(live, s)
		}
	}
	return live
}

// TestSubscribeWithCard_SamePaidTierRefusedBeforeCharge is the money regression:
// a subject who already pays cannot buy a duplicate of the same tier, and the refusal
// lands BEFORE the card is touched.
func TestSubscribeWithCard_SamePaidTierRefusedBeforeCharge(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("subsecond")
	m := squareMock("cust_1", "ccof_1", "sqpay_1")
	withFakeSquare(t, m)

	// A real first purchase, through the real endpoint.
	if resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"dev"}`, map[string]string{"X-Idempotency-Key": "attempt-1"}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("first subscribe status=%d, want 201", resp.StatusCode)
	}
	chargesAfterFirst := m.chargeCalls

	// Attempt to buy the SAME plan again with a new idempotency key so it tests the held check rather than replaying.
	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok2","planId":"dev"}`, map[string]string{"X-Idempotency-Key": "attempt-2"})
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("second subscribe status=%d body=%s, want 409 — buying the same paid tier again must be refused",
			resp.StatusCode, string(body))
	}
	// The refusal has to name what the customer already has, or they cannot act on it.
	if !strings.Contains(string(body), "dev") {
		t.Errorf("409 body %s does not name the existing plan", string(body))
	}

	// THE POINT: no money moved and no second subscription exists.
	if m.chargeCalls != chargesAfterFirst {
		t.Errorf("charge calls=%d, want %d — the card was touched on a refused sale",
			m.chargeCalls, chargesAfterFirst)
	}
	db := datastore.New(org.Namespaced(ctx))
	if got := liveSubs(t, db, "subsecond"); len(got) != 1 {
		t.Errorf("live subscriptions=%d, want 1 — the subject now holds two renewing tiers", len(got))
	}
}

// TestSubscribeWithCard_UpgradePaidTierCancelsOldAndChargesNew tests upgrading
// an existing paid subscription: customer holding 'dev' upgrades to 'max-5x',
// charges the new tier, cancels the old 'dev' subscription, and leaves 1 active subscription.
func TestSubscribeWithCard_UpgradePaidTierCancelsOldAndChargesNew(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("subupgrade")
	m := squareMock("cust_up", "ccof_up", "sqpay_up")
	withFakeSquare(t, m)

	if resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"dev"}`, nil); resp.StatusCode != http.StatusCreated {
		t.Fatalf("first subscribe status=%d, want 201", resp.StatusCode)
	}
	chargesAfterFirst := m.chargeCalls

	// Upgrade to 'max-5x'
	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok2","planId":"max-5x"}`, nil)
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("upgrade status=%d body=%s, want 201", resp.StatusCode, string(body))
	}
	if m.chargeCalls != chargesAfterFirst+1 {
		t.Errorf("charge calls=%d, want %d", m.chargeCalls, chargesAfterFirst+1)
	}

	db := datastore.New(org.Namespaced(ctx))
	live := liveSubs(t, db, "subupgrade")
	if len(live) != 1 {
		t.Fatalf("live subscriptions=%d, want 1 after upgrade", len(live))
	}
	if live[0].PlanId != "max-5x" && live[0].Plan.Slug != "max-5x" {
		t.Errorf("active subscription plan=%q, want 'max-5x'", live[0].PlanId)
	}
}

// TestSubscribeWithCard_FreeTierDoesNotBlockPaying: the main funnel. A free
// subscription bills nothing, so it can never be the thing a second sale would
// double-charge — and a customer on the free tier is exactly who needs this endpoint.
func TestSubscribeWithCard_FreeTierDoesNotBlockPaying(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("subfree")
	m := squareMock("cust_f", "ccof_f", "sqpay_f")
	withFakeSquare(t, m)

	db := datastore.New(org.Namespaced(ctx))
	// Payment-backed on purpose: it is the PRICE that excludes it, not the absence
	// of a provider.
	liveSub(t, db, "subfree", "dns-free", string(processor.Square))

	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"dev"}`, nil)
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s, want 201 — a free tier must not block the customer who is finally paying",
			resp.StatusCode, string(body))
	}
	if parentSub(t, db, "subfree", "dev") == nil {
		t.Error("no 'pro' subscription created")
	}
}

// TestSubscribeWithCard_UnpaidInternalRowDoesNotBlockPaying: a zero-payment
// internal Active row (what CreateBillingSubscription starts) collects nothing, so
// it cannot double-charge — and counting it would trap its holder outside the only
// endpoint that can fix it.
func TestSubscribeWithCard_UnpaidInternalRowDoesNotBlockPaying(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("subunpaid")
	m := squareMock("cust_u", "ccof_u", "sqpay_u")
	withFakeSquare(t, m)

	db := datastore.New(org.Namespaced(ctx))
	// A PAID slug with no provider and no invoice — not payment-backed.
	held := liveSub(t, db, "subunpaid", "dev", "internal")
	if subscriptionPaymentBacked(held) {
		t.Fatal("seeded row must not be payment-backed, or this test asserts nothing")
	}

	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"dev"}`, nil)
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s, want 201 — an unpaid internal row must not block paying",
			resp.StatusCode, string(body))
	}
}

// TestSubscribeWithCard_Max50xAnnual tests purchasing Max 50x yearly,
// charging $5,000 (500000 cents).
func TestSubscribeWithCard_Max50xAnnual(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("sub50x")
	m := squareMock("cust_50x", "ccof_50x", "sqpay_50x")
	withFakeSquare(t, m)

	pdb := plan.AuthorityDB(ctx)
	p := plan.New(pdb)
	p.Slug = "max-50x"
	p.Name = "Max 50x"
	p.Price = 50000
	p.AnnualTotal = 500000
	if err := p.Create(); err != nil {
		t.Fatalf("create plan max-50x: %v", err)
	}

	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"max-50x","interval":"year"}`, nil)
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s, want 201", resp.StatusCode, string(body))
	}
	body := jsonBody(t, resp)
	if got := int64(body["amountCents"].(float64)); got != 500000 {
		t.Fatalf("amountCents=%d, want 500000 ($5000)", got)
	}
	if got := body["interval"]; got != "year" {
		t.Errorf("interval=%v, want 'year'", got)
	}
}

// TestSubscribeWithCard_SwitchIntervalMonthlyToYearly tests that an existing monthly subscriber
// can switch to annual billing, canceling the old monthly sub and starting annual.
func TestSubscribeWithCard_SwitchIntervalMonthlyToYearly(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("subswitch")
	m := squareMock("cust_sw", "ccof_sw", "sqpay_sw")
	withFakeSquare(t, m)

	// Buy dev monthly first
	if resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"dev"}`, map[string]string{"X-Idempotency-Key": "m-1"}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("monthly subscribe status=%d, want 201", resp.StatusCode)
	}

	// Switch to dev annual
	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok2","planId":"dev","interval":"year"}`, map[string]string{"X-Idempotency-Key": "y-1"})
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("annual switch status=%d body=%s, want 201", resp.StatusCode, string(body))
	}
	body := jsonBody(t, resp)
	if got := int64(body["amountCents"].(float64)); got != 20000 {
		t.Fatalf("amountCents=%d, want 20000 ($200)", got)
	}

	db := datastore.New(org.Namespaced(ctx))
	live := liveSubs(t, db, "subswitch")
	if len(live) != 1 {
		t.Fatalf("live subscriptions=%d, want 1 after annual switch", len(live))
	}
	if live[0].Plan.Interval != types.Yearly {
		t.Errorf("active subscription interval=%v, want 'year'", live[0].Plan.Interval)
	}
}
