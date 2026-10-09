package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/api/promo"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/mail"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/coupon"
	"github.com/hanzoai/commerce/models/couponredemption"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A plan coupon at checkout: 50% off the FIRST month of one plan purchase,
// renewals at the catalog price, and nothing about any other price moves.

// warmCouponNS opens the platform namespace's coupon kinds once, so the writes a
// test drives and the reads it checks share one handle, and seeds the plan
// authority from the catalog, as every boot does: a coupon may only name a paid
// plan the authority sells.
func warmCouponNS(t *testing.T, ctx context.Context) {
	t.Helper()
	db := datastore.New(nscontext.WithNamespace(ctx, "admin"))
	_, _ = coupon.Query(db).Count()
	_, _ = couponredemption.Query(db).Count()
	_, _ = billingevent.Query(db).Count()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed plans: %v", err)
	}
}

// planCoupon stores a plan coupon through the SuperAdmin core.
func planCoupon(t *testing.T, ctx context.Context, code string, spec promo.CouponSpec) {
	t.Helper()
	if _, err := promo.SetCoupon(ctx, code, spec, "z@hanzo.ai"); err != nil {
		t.Fatalf("SetCoupon(%s): %v", code, err)
	}
}

// fiftyOff is the 50OFF coupon the owner asked for: 50%, open now, a day long,
// five uses, on the three paid personal tiers.
func fiftyOff(t *testing.T, ctx context.Context) {
	now := time.Now().UTC()
	start, end := now.Add(-time.Minute), now.Add(24*time.Hour)
	planCoupon(t, ctx, "50OFF", promo.CouponSpec{
		Percent: 50, Start: &start, End: &end, Limit: 5,
		Plans: []string{"dev", "max-5x", "max-20x"}, Enabled: true,
	})
}

func couponUses(t *testing.T, ctx context.Context, code string) int {
	t.Helper()
	v, err := promo.ReadCoupon(ctx, code)
	if err != nil {
		t.Fatalf("read coupon %s: %v", code, err)
	}
	return v.Used
}

// dueNow moves a subscription's held period into the past, so the next renewal
// bills the period after it.
func dueNow(t *testing.T, db *datastore.Datastore, id string) *subscription.Subscription {
	t.Helper()
	s := subscription.New(db)
	if err := s.GetById(id); err != nil {
		t.Fatalf("load subscription: %v", err)
	}
	s.PeriodEnd = time.Now().AddDate(0, 0, -25)
	s.PeriodStart = s.PeriodEnd.AddDate(0, -1, 0)
	if err := s.Update(); err != nil {
		t.Fatalf("move period: %v", err)
	}
	return s
}

// The whole contract in one sale: the card is charged half, the first invoice
// says so and equals the charge, the subscription carries no discount, the use
// is recorded, and the renewal bills the full catalog price.
func TestSubscribeCoupon_FirstMonthHalfRenewalFull(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCouponNS(t, ctx)
	fiftyOff(t, ctx)
	org := moneyOrg("cp-first")
	m := squareMock("cust_cf", "ccof_cf", "sqpay_cf")
	withFakeSquare(t, m)

	list := lookupPlan("dev").Price
	half := list - engine.DiscountCents(list, 50)

	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"dev","coupon":" 50off "}`, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", resp.StatusCode, bodyOf(resp))
	}
	out := jsonBody(t, resp)
	if int64(out["amountCents"].(float64)) != half || out["coupon"] != "50OFF" || out["couponPercent"].(float64) != 50 {
		t.Fatalf("receipt %+v, want amountCents %d with coupon 50OFF at 50%%", out, half)
	}
	if m.chargeCalls != 1 || m.lastChargeAmount != half {
		t.Fatalf("charged %d time(s) for %d, want once for %d", m.chargeCalls, m.lastChargeAmount, half)
	}

	db := datastore.New(org.Namespaced(ctx))
	sub := parentSub(t, db, "cp-first", "dev")
	if sub == nil {
		t.Fatal("no subscription")
	}
	if sub.DiscountPercent != 0 || sub.DiscountName != "" {
		t.Fatalf("subscription carries discount %d%% %q; the coupon must not reach renewals", sub.DiscountPercent, sub.DiscountName)
	}
	if sub.Metadata["coupon"] != "50OFF" {
		t.Fatalf("subscription metadata %v, want the coupon noted", sub.Metadata)
	}
	invs := invoicesForSub(t, db, sub.Id())
	if len(invs) != 1 {
		t.Fatalf("invoices=%d, want 1", len(invs))
	}
	first := invs[0]
	if first.AmountDue != half || first.AmountPaid != half || first.Discount != list-half {
		t.Fatalf("first invoice due=%d paid=%d discount=%d, want %d/%d/%d", first.AmountDue, first.AmountPaid, first.Discount, half, half, list-half)
	}
	if first.DiscountName != "50OFF — 50% off first month" {
		t.Fatalf("first invoice discount name %q", first.DiscountName)
	}
	if first.Metadata["coupon"] != "50OFF" {
		t.Fatalf("first invoice metadata %v, want the coupon recorded", first.Metadata)
	}
	if n := couponUses(t, ctx, "50OFF"); n != 1 {
		t.Fatalf("coupon used %d, want 1", n)
	}
	if !promo.Redeemed(ctx, "50OFF", "cp-first") {
		t.Fatal("no redemption recorded for the org")
	}

	// The renewal: full catalog price, no discount, nothing about the coupon.
	s := dueNow(t, db, sub.Id())
	inv, _, err := engine.RenewSubscription(ctx, db, s, prepaidFor(ctx, org), chargeProviderForOrg(org))
	if err != nil || inv == nil {
		t.Fatalf("renew: inv=%v err=%v", inv, err)
	}
	if m.chargeCalls != 2 || m.lastChargeAmount != list {
		t.Fatalf("renewal charged %d (calls %d), want the catalog %d", m.lastChargeAmount, m.chargeCalls, list)
	}
	if inv.AmountDue != list || inv.Discount != 0 || inv.DiscountName != "" || inv.Metadata["coupon"] != nil {
		t.Fatalf("renewal invoice due=%d discount=%d %q meta=%v, want %d undiscounted", inv.AmountDue, inv.Discount, inv.DiscountName, inv.Metadata, list)
	}
}

// A promo and a coupon stack: the coupon comes off what the promo left, the
// charge equals the first invoice to the cent, and the renewal keeps the promo
// the subscription carries and drops the coupon.
func TestSubscribeCoupon_StacksOnPromo(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCouponNS(t, ctx)
	fiftyOff(t, ctx)
	org := moneyOrg("cp-stack")
	m := squareMock("cust_cs", "ccof_cs", "sqpay_cs")
	withFakeSquare(t, m)

	list := lookupPlan("max-5x").Price
	pr := &promo.Promo{PercentOff: 20, Active: true}
	want := engine.Discounted(list, 20, 50)

	sale, err := Subscribe(ctx, org, SubscribeIn{
		SourceID: "cnon:ok", PlanID: "max-5x", Subject: "cp-stack", Promo: pr, Coupon: "50OFF",
	})
	if err != nil {
		t.Fatalf("sale: %v", err)
	}
	if sale.AmountCents != want || m.lastChargeAmount != want {
		t.Fatalf("charged %d (receipt %d), want %d", m.lastChargeAmount, sale.AmountCents, want)
	}
	db := datastore.New(org.Namespaced(ctx))
	sub := parentSub(t, db, "cp-stack", "max-5x")
	first := invoicesForSub(t, db, sub.Id())[0]
	if first.AmountDue != want || first.DiscountName != "20% off; 50OFF — 50% off first month" {
		t.Fatalf("first invoice due=%d name=%q, want %d with both discounts named", first.AmountDue, first.DiscountName, want)
	}
	if sub.DiscountPercent != 20 {
		t.Fatalf("subscription discount %d%%, want the promo's 20", sub.DiscountPercent)
	}

	s := dueNow(t, db, sub.Id())
	inv, _, err := engine.RenewSubscription(ctx, db, s, prepaidFor(ctx, org), chargeProviderForOrg(org))
	if err != nil || inv == nil {
		t.Fatalf("renew: %v", err)
	}
	if renewal := engine.Discounted(list, 20); inv.AmountDue != renewal || m.lastChargeAmount != renewal {
		t.Fatalf("renewal due=%d charged=%d, want %d (promo only)", inv.AmountDue, m.lastChargeAmount, renewal)
	}
}

// Every coupon that cannot be used refuses the sale with a 4xx naming why,
// before any card is vaulted or charged, and opens nothing.
func TestSubscribeCoupon_Refusals(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCouponNS(t, ctx)
	fiftyOff(t, ctx)
	now := time.Now().UTC()
	past, future := now.Add(-time.Hour), now.Add(time.Hour)
	planCoupon(t, ctx, "OLD", promo.CouponSpec{Percent: 50, End: &past, Plans: []string{"dev"}, Enabled: true})
	planCoupon(t, ctx, "SOON", promo.CouponSpec{Percent: 50, Start: &future, Plans: []string{"dev"}, Enabled: true})
	planCoupon(t, ctx, "DARK", promo.CouponSpec{Percent: 50, Plans: []string{"dev"}, Enabled: false})
	planCoupon(t, ctx, "MAXONLY", promo.CouponSpec{Percent: 50, Plans: []string{"max-5x"}, Enabled: true})
	planCoupon(t, ctx, "ONEUSE", promo.CouponSpec{Percent: 50, Limit: 1, Plans: []string{"dev"}, Enabled: true})
	planCoupon(t, ctx, "FREEBIE", promo.CouponSpec{Percent: 100, Plans: []string{"dev"}, Enabled: true})
	if _, err := promo.Reserve(ctx, "ONEUSE", "someone-else", "fp:else", now); err != nil {
		t.Fatalf("spend ONEUSE: %v", err)
	}

	cases := []struct {
		name, body, says string
		status           int
	}{
		{"expired", `{"sourceId":"cnon:ok","planId":"dev","coupon":"OLD"}`, "expired", 400},
		{"not started", `{"sourceId":"cnon:ok","planId":"dev","coupon":"SOON"}`, "not valid", 400},
		{"disabled", `{"sourceId":"cnon:ok","planId":"dev","coupon":"DARK"}`, "not valid", 400},
		{"exhausted", `{"sourceId":"cnon:ok","planId":"dev","coupon":"ONEUSE"}`, "fully redeemed", 400},
		{"unknown", `{"sourceId":"cnon:ok","planId":"dev","coupon":"NOSUCH"}`, "not valid", 400},
		{"malformed", `{"sourceId":"cnon:ok","planId":"dev","coupon":"50 OFF"}`, "not valid", 400},
		{"wrong plan", `{"sourceId":"cnon:ok","planId":"dev","coupon":"MAXONLY"}`, "does not apply", 400},
		{"not a covered plan", `{"sourceId":"cnon:ok","planId":"team","quantity":2,"coupon":"50OFF"}`, "does not apply", 400},
		{"annual", `{"sourceId":"cnon:ok","planId":"dev","interval":"year","coupon":"50OFF"}`, "monthly", 400},
		{"prepaid", `{"sourceId":"credits","planId":"dev","coupon":"50OFF"}`, "card", 400},
		{"free", `{"sourceId":"cnon:ok","planId":"free","coupon":"50OFF"}`, "free", 400},
		{"nothing left to charge", `{"sourceId":"cnon:ok","planId":"dev","coupon":"FREEBIE"}`, "nothing to charge", 400},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name := fmt.Sprintf("cp-refuse-%d", i)
			org := moneyOrg(name)
			m := squareMock("cust_"+name, "ccof_"+name, "sqpay_"+name)
			withFakeSquare(t, m)

			resp := invokeSubscribeCard(org, ctx, tc.body, nil)
			body := bodyOf(resp)
			if resp.StatusCode != tc.status || !strings.Contains(body, tc.says) {
				t.Fatalf("status=%d body=%s, want %d saying %q", resp.StatusCode, body, tc.status, tc.says)
			}
			if m.chargeCalls != 0 || m.createCustomerCalls != 0 {
				t.Fatalf("refused sale touched the card: charges=%d vaults=%d", m.chargeCalls, m.createCustomerCalls)
			}
			db := datastore.New(org.Namespaced(ctx))
			subs := make([]*subscription.Subscription, 0)
			_, _ = subscription.Query(db).Filter("UserId=", name).GetAll(&subs)
			if len(subs) != 0 {
				t.Fatalf("refused sale opened %d subscription(s)", len(subs))
			}
		})
	}
	if n := couponUses(t, ctx, "50OFF"); n != 0 {
		t.Fatalf("refused sales spent %d use(s) of 50OFF", n)
	}
}

// An org redeems a code once, whichever of its accounts buys; a card redeems a
// code once, whichever org it pays for. Both refusals are a 409 before any money.
func TestSubscribeCoupon_OncePerOrgAndCard(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCouponNS(t, ctx)
	fiftyOff(t, ctx)
	m := squareMock("cust_oc", "ccof_oc", "sqpay_oc")
	m.vaultCard = processor.Card{Brand: "VISA", Last4: "4242", ExpMonth: 12, ExpYear: 2030, Fingerprint: "fp_same_card"}
	withFakeSquare(t, m)

	a := moneyOrg("cp-org-a")
	if r := invokeSubscribeCard(a, ctx, `{"sourceId":"cnon:a","planId":"dev","coupon":"50OFF"}`, nil); r.StatusCode != http.StatusCreated {
		t.Fatalf("first sale status=%d %s", r.StatusCode, bodyOf(r))
	}

	// Another account in the same org, with another card: the org has redeemed.
	m.vaultCard = processor.Card{Brand: "VISA", Last4: "1111", ExpMonth: 1, ExpYear: 2031, Fingerprint: "fp_other_card"}
	vaults := m.createCustomerCalls
	r := invokeSubscribeCard(a, ctx, `{"sourceId":"cnon:a2","planId":"dev","userId":"cp-org-a/member","coupon":"50OFF"}`, nil)
	if body := bodyOf(r); r.StatusCode != http.StatusConflict || !strings.Contains(body, "organization has already redeemed") {
		t.Fatalf("same org again: status=%d %s, want 409", r.StatusCode, body)
	}
	if m.createCustomerCalls != vaults || m.chargeCalls != 1 {
		t.Fatalf("the org refusal touched a card: vaults %d→%d charges %d", vaults, m.createCustomerCalls, m.chargeCalls)
	}

	// Another org, the first org's card: the card has redeemed.
	m.vaultCard = processor.Card{Brand: "VISA", Last4: "4242", ExpMonth: 12, ExpYear: 2030, Fingerprint: "fp_same_card"}
	b := moneyOrg("cp-org-b")
	r = invokeSubscribeCard(b, ctx, `{"sourceId":"cnon:b","planId":"dev","coupon":"50OFF"}`, nil)
	if body := bodyOf(r); r.StatusCode != http.StatusConflict || !strings.Contains(body, "card has already redeemed") {
		t.Fatalf("same card again: status=%d %s, want 409", r.StatusCode, body)
	}
	if m.chargeCalls != 1 {
		t.Fatalf("the card refusal charged: %d charges", m.chargeCalls)
	}
	if pms := pmsFor(t, datastore.New(b.Namespaced(ctx)), "cp-org-b"); len(pms) != 0 {
		t.Fatalf("the refused card stayed saved for org b: %d row(s)", len(pms))
	}
	if !m.removeCalled {
		t.Fatal("the card vaulted for the refused sale was not detached")
	}
	if n := couponUses(t, ctx, "50OFF"); n != 1 {
		t.Fatalf("coupon used %d, want 1", n)
	}

	// The same org B with its own card, no coupon: an ordinary sale.
	m.vaultCard = processor.Card{Brand: "VISA", Last4: "5555", ExpMonth: 2, ExpYear: 2032, Fingerprint: "fp_b_card"}
	if r := invokeSubscribeCard(b, ctx, `{"sourceId":"cnon:b2","planId":"dev"}`, nil); r.StatusCode != http.StatusCreated {
		t.Fatalf("plain sale after a refused coupon: status=%d %s", r.StatusCode, bodyOf(r))
	}
}

// A retry of a coupon sale replays its receipt and spends nothing more; a
// different coupon (or none) is a different purchase, never a replay of it.
func TestSubscribeCoupon_Idempotent(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCouponNS(t, ctx)
	fiftyOff(t, ctx)
	org := moneyOrg("cp-idem")
	m := squareMock("cust_ci", "ccof_ci", "sqpay_ci")
	withFakeSquare(t, m)

	// No key: an unknown code refuses and holds nothing, so the next attempt
	// with the right code is a sale, not a replay of the refusal.
	if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:1","planId":"dev","coupon":"TYPO"}`, nil); r.StatusCode != 400 {
		t.Fatalf("typo status=%d, want 400", r.StatusCode)
	}
	hdr := map[string]string{"X-Idempotency-Key": "coupon-sale-1"}
	body := `{"sourceId":"cnon:2","planId":"dev","coupon":"50OFF"}`
	r1 := invokeSubscribeCard(org, ctx, body, hdr)
	if r1.StatusCode != http.StatusCreated {
		t.Fatalf("sale status=%d %s", r1.StatusCode, bodyOf(r1))
	}
	b1 := bodyOf(r1)
	r2 := invokeSubscribeCard(org, ctx, body, hdr)
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("retry status=%d, want 200 replay", r2.StatusCode)
	}
	if b2 := bodyOf(r2); b2 != b1 {
		t.Fatalf("replay differs:\n%s\n%s", b1, b2)
	}
	if m.chargeCalls != 1 || couponUses(t, ctx, "50OFF") != 1 {
		t.Fatalf("retry charged %d / used %d, want 1/1", m.chargeCalls, couponUses(t, ctx, "50OFF"))
	}

	// No key, no coupon, inside the window: a different purchase — refused because
	// the account already pays, never handed the discounted receipt.
	if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:3","planId":"dev"}`, nil); r.StatusCode != http.StatusConflict {
		t.Fatalf("coupon-less retry status=%d, want 409 (a different purchase)", r.StatusCode)
	}
}

// A declined card gives the reserved use back whole; a later sale may take it.
func TestSubscribeCoupon_DeclineReleasesTheUse(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCouponNS(t, ctx)
	fiftyOff(t, ctx)
	org := moneyOrg("cp-decline")
	m := squareMock("cust_cd", "ccof_cd", "sqpay_cd")
	m.vaultCard = processor.Card{Brand: "VISA", Last4: "4242", ExpMonth: 12, ExpYear: 2030, Fingerprint: "fp_declined"}
	m.chargeErr = errors.New("CARD_DECLINED")
	withFakeSquare(t, m)

	body := `{"sourceId":"cnon:d","planId":"dev","coupon":"50OFF"}`
	if r := invokeSubscribeCard(org, ctx, body, map[string]string{"X-Idempotency-Key": "d-1"}); r.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("decline status=%d %s, want 402", r.StatusCode, bodyOf(r))
	}
	if n := couponUses(t, ctx, "50OFF"); n != 0 {
		t.Fatalf("a declined sale kept %d use(s)", n)
	}
	if promo.Redeemed(ctx, "50OFF", "cp-decline") {
		t.Fatal("a declined sale left the org marked as redeemed")
	}

	m.chargeErr = nil
	if r := invokeSubscribeCard(org, ctx, body, map[string]string{"X-Idempotency-Key": "d-2"}); r.StatusCode != http.StatusCreated {
		t.Fatalf("retry after decline status=%d %s", r.StatusCode, bodyOf(r))
	}
	if n := couponUses(t, ctx, "50OFF"); n != 1 {
		t.Fatalf("coupon used %d, want 1", n)
	}
}

// Buyers racing for a coupon's last uses: exactly Limit sales are charged.
func TestSubscribeCoupon_ConcurrentSalesNeverExceedLimit(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCouponNS(t, ctx)
	const limit, buyers = 3, 12
	planCoupon(t, ctx, "RUSH", promo.CouponSpec{Percent: 50, Limit: limit, Plans: []string{"dev"}, Enabled: true})
	m := squareMock("", "", "sqpay_rush")
	withFakeSquare(t, m)

	orgs := make([]*organization.Organization, buyers)
	methods := make([]string, buyers)
	for i := range orgs {
		name := fmt.Sprintf("cp-rush-%d", i)
		orgs[i] = moneyOrg(name)
		db := datastore.New(orgs[i].Namespaced(ctx))
		pm := paymentmethod.New(db)
		pm.CustomerId, pm.UserId, pm.Type = name, name, "card"
		pm.ProviderRef = "ccof_" + name
		pm.ProviderType = string(processor.Square)
		pm.Metadata = map[string]interface{}{"squareCustomerId": "cust_" + name, "fingerprint": "fp_" + name}
		if err := pm.Create(); err != nil {
			t.Fatalf("seed card: %v", err)
		}
		methods[i] = pm.Id()
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	sold, refused := 0, 0
	start := make(chan struct{})
	for i := range orgs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := Subscribe(ctx, orgs[i], SubscribeIn{
				MethodID: methods[i], PlanID: "dev", Subject: orgs[i].Name, Coupon: "RUSH",
			})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				sold++
			case IsSaleRefused(err) && strings.Contains(err.Error(), "fully redeemed"):
				refused++
			default:
				t.Errorf("buyer %d: %v", i, err)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	if sold != limit || refused != buyers-limit {
		t.Fatalf("sold %d refused %d, want %d and %d", sold, refused, limit, buyers-limit)
	}
	if m.chargeCalls != limit {
		t.Fatalf("charged %d cards, want %d", m.chargeCalls, limit)
	}
	if n := couponUses(t, ctx, "RUSH"); n != limit {
		t.Fatalf("coupon used %d, want %d", n, limit)
	}
}

// sent is one message a fake mail rail delivered.
type sent struct {
	to            []string
	subject, body string
}

type senderFunc func(ctx context.Context, to []string, subject, body string) error

func (f senderFunc) Send(ctx context.Context, to []string, subject, body string) error {
	return f(ctx, to, subject, body)
}

// Every paid sale tells the configured operator once — with the coupon when
// there is one — and the mail rail never holds a sale up.
func TestSaleNotice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCouponNS(t, ctx)
	fiftyOff(t, ctx)
	m := squareMock("cust_sn", "ccof_sn", "sqpay_sn")
	withFakeSquare(t, m)

	got := make(chan sent, 8)
	mail.Set(senderFunc(func(_ context.Context, to []string, subject, body string) error {
		got <- sent{to, subject, body}
		return nil
	}))
	t.Cleanup(func() { mail.Set(nil) })
	next := func() (sent, bool) {
		select {
		case s := <-got:
			return s, true
		case <-time.After(5 * time.Second):
			return sent{}, false
		}
	}
	quiet := func() {
		select {
		case s := <-got:
			t.Fatalf("an extra notice was sent: %q", s.subject)
		case <-time.After(300 * time.Millisecond):
		}
	}

	// Unset: nobody is told.
	t.Setenv(saleNotifyEnv, "")
	if r := invokeSubscribeCard(moneyOrg("cp-note-0"), ctx, `{"sourceId":"cnon:0","planId":"dev"}`, nil); r.StatusCode != 201 {
		t.Fatalf("sale status=%d", r.StatusCode)
	}
	quiet()

	t.Setenv(saleNotifyEnv, " ops@example.test, not an address ,second@example.test")
	dev := lookupPlan("dev")
	hdr := map[string]string{"X-Idempotency-Key": "note-1"}
	org := moneyOrg("cp-note-1")
	if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:1","planId":"dev","coupon":"50OFF"}`, hdr); r.StatusCode != 201 {
		t.Fatalf("coupon sale status=%d %s", r.StatusCode, bodyOf(r))
	}
	s, ok := next()
	if !ok {
		t.Fatal("no notice for a paid coupon sale")
	}
	if strings.Join(s.to, ",") != "ops@example.test,second@example.test" {
		t.Fatalf("sent to %v", s.to)
	}
	half := dev.Price - engine.DiscountCents(dev.Price, 50)
	for _, want := range []string{
		"cp-note-1", "buyer@acme.test", dev.Name + " (dev)", "month",
		"Charged:        " + cents(half, "USD"), "List price:     " + cents(dev.Price, "USD"),
		"50OFF — 50% off first month", "Mode:           live",
	} {
		if !strings.Contains(s.body, want) {
			t.Errorf("notice body lacks %q:\n%s", want, s.body)
		}
	}
	if !strings.Contains(s.subject, "coupon 50OFF") || !strings.Contains(s.subject, "cp-note-1") {
		t.Errorf("notice subject %q", s.subject)
	}

	// The replay is not a sale.
	if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:1","planId":"dev","coupon":"50OFF"}`, hdr); r.StatusCode != 200 {
		t.Fatalf("replay status=%d", r.StatusCode)
	}
	quiet()

	// A plain sale is told too, with no coupon.
	if r := invokeSubscribeCard(moneyOrg("cp-note-2"), ctx, `{"sourceId":"cnon:2","planId":"max-5x"}`, nil); r.StatusCode != 201 {
		t.Fatalf("plain sale status=%d", r.StatusCode)
	}
	if s, ok = next(); !ok || !strings.Contains(s.body, "Coupon:         none") {
		t.Fatalf("plain sale notice ok=%v body:\n%s", ok, s.body)
	}

	// A mail rail that never answers, then one that fails: the sale answers at
	// once either way.
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	mail.Set(senderFunc(func(ctx context.Context, _ []string, _, _ string) error {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return ctx.Err()
	}))
	began := time.Now()
	if r := invokeSubscribeCard(moneyOrg("cp-note-3"), ctx, `{"sourceId":"cnon:3","planId":"dev"}`, nil); r.StatusCode != 201 {
		t.Fatalf("sale behind a stuck mail rail status=%d", r.StatusCode)
	}
	if d := time.Since(began); d > 3*time.Second {
		t.Fatalf("a stuck mail rail held the sale %v", d)
	}
	mail.Set(senderFunc(func(context.Context, []string, string, string) error { return errors.New("rail down") }))
	if r := invokeSubscribeCard(moneyOrg("cp-note-4"), ctx, `{"sourceId":"cnon:4","planId":"dev"}`, nil); r.StatusCode != 201 {
		t.Fatalf("sale behind a failing mail rail status=%d", r.StatusCode)
	}
	// A mail rail that PANICS is the notice's failure and nobody else's: unrecovered,
	// it would take this test binary down, as it would the binary that embeds commerce.
	panicked := make(chan struct{})
	mail.Set(senderFunc(func(context.Context, []string, string, string) error {
		close(panicked)
		panic("rail exploded")
	}))
	if r := invokeSubscribeCard(moneyOrg("cp-note-5"), ctx, `{"sourceId":"cnon:5","planId":"dev"}`, nil); r.StatusCode != 201 {
		t.Fatalf("sale behind a panicking mail rail status=%d", r.StatusCode)
	}
	select {
	case <-panicked:
	case <-time.After(5 * time.Second):
		t.Fatal("the panicking mail rail was never called")
	}
	time.Sleep(200 * time.Millisecond) // the recover runs; an unrecovered panic ends the binary here
}

func listPlansWith(t *testing.T, query string) (string, []map[string]any) {
	t.Helper()
	a := zip.New(zip.Config{DisableStartupMessage: true})
	// The route as it is mounted: the catalog is cached at the edge for an hour.
	a.Raw(http.MethodGet, "/v1/billing/plans", middleware.CachePublic(3600), ListPlans)
	resp, err := a.Test(httptest.NewRequest(http.MethodGet, "/v1/billing/plans"+query, nil))
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("GET plans%s: err=%v status=%v", query, err, statusOf(resp))
	}
	// A coupon quote is never cached; the plain catalog keeps its public caching.
	quoted := strings.Contains(query, "coupon=") && !strings.HasSuffix(query, "coupon=")
	cc, cdn := resp.Header.Get("Cache-Control"), resp.Header.Get("CDN-Cache-Control")
	if quoted && (cc != "no-store" || cdn != "no-store") {
		t.Fatalf("GET plans%s: Cache-Control %q CDN-Cache-Control %q, want no-store for a coupon quote", query, cc, cdn)
	}
	if !quoted && !strings.HasPrefix(cc, "public") {
		t.Fatalf("GET plans%s: Cache-Control %q, want the catalog's public caching", query, cc)
	}
	raw := bodyOf(resp)
	var rows []map[string]any
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		t.Fatalf("decode plans: %v", err)
	}
	return raw, rows
}

func bySlug(rows []map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, r := range rows {
		if s, ok := r["slug"].(string); ok {
			out[s] = r
		}
	}
	return out
}

// The plans quote: no code is the catalog exactly as before; a good code
// annotates the plans it covers with the first month's price; a bad one names
// why on every row and prices nothing.
func TestPlansCouponQuote(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCouponNS(t, ctx)

	before, rows := listPlansWith(t, "")
	for _, r := range rows {
		for k := range r {
			if strings.HasPrefix(k, "coupon") {
				t.Fatalf("a catalog read without a code carries %q", k)
			}
		}
	}

	fiftyOff(t, ctx)
	now := time.Now().UTC()
	past := now.Add(-time.Hour)
	planCoupon(t, ctx, "OLD", promo.CouponSpec{Percent: 50, End: &past, Plans: []string{"dev"}, Enabled: true})
	planCoupon(t, ctx, "ONEUSE", promo.CouponSpec{Percent: 50, Limit: 1, Plans: []string{"dev"}, Enabled: true})
	if _, err := promo.Reserve(ctx, "ONEUSE", "someone", "fp:x", now); err != nil {
		t.Fatalf("spend ONEUSE: %v", err)
	}

	if after, _ := listPlansWith(t, ""); after != before {
		t.Fatal("storing coupons changed the catalog read without a code")
	}
	if empty, _ := listPlansWith(t, "?coupon="); empty != before {
		t.Fatal("an empty code changed the catalog")
	}

	v, _ := promo.ReadCoupon(ctx, "50OFF")
	_, rows = listPlansWith(t, "?coupon=50off")
	plans := bySlug(rows)
	for _, slug := range []string{"dev", "max-5x", "max-20x"} {
		r := plans[slug]
		price := int64(r["price"].(float64))
		if r["couponCode"] != "50OFF" || r["couponPercent"] != float64(50) ||
			int64(r["couponFirstCents"].(float64)) != price-engine.DiscountCents(price, 50) ||
			r["couponEnds"] != v.End.UTC().Format(time.RFC3339) || r["couponError"] != nil {
			t.Errorf("%s quote %v", slug, r)
		}
		if int64(r["price"].(float64)) != lookupPlan(slug).Price {
			t.Errorf("%s list price moved to %v", slug, r["price"])
		}
	}
	for _, slug := range []string{"free", "team", "enterprise"} {
		if r := plans[slug]; r["couponError"] != promo.CouponNotApplicable || r["couponCode"] != nil {
			t.Errorf("%s under 50OFF: %v", slug, r)
		}
	}

	for code, reason := range map[string]string{"NOSUCH": promo.CouponUnknown, "OLD": promo.CouponExpired, "ONEUSE": promo.CouponExhausted, "bad code": promo.CouponUnknown} {
		_, rows = listPlansWith(t, "?coupon="+strings.ReplaceAll(code, " ", "%20"))
		for _, r := range rows {
			if r["couponError"] != reason || r["couponCode"] != nil || r["couponFirstCents"] != nil {
				t.Errorf("%s: row %v, want couponError %s and no quote", code, r["slug"], reason)
			}
		}
	}
	if n := couponUses(t, ctx, "50OFF"); n != 0 {
		t.Fatalf("a quote spent %d use(s)", n)
	}
}
