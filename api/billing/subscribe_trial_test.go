package billing

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/api/promo"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/mail"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	types "github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A card-verified free trial: a plan with trialPeriodDays sold by card to a buyer
// and a card that have not had it charges nothing today, vaults the card, and
// opens the subscription trialing; the renewal sweep charges the card for the
// first paid period when the trial ends.

const trialCents = 2900

// trialPlan writes a monthly plan the catalog does not publish, priced at cents
// with a free trial of days, the way the admin CRUD writes it.
func trialPlan(t *testing.T, ctx context.Context, slug string, cents int64, days int) {
	t.Helper()
	p := plan.New(datastore.New(nscontext.WithNamespace(ctx, plan.Namespace)))
	p.Slug, p.Name = slug, slug
	p.Price, p.Currency = currency.Cents(cents), currency.USD
	p.Interval, p.IntervalCount = types.Monthly, 1
	p.TrialPeriodDays = days
	p.AdminEdited = true
	if err := p.Create(); err != nil {
		t.Fatalf("create plan %s: %v", slug, err)
	}
}

// trialSquare is a Square fake whose vault reports a card with fingerprint fp.
func trialSquare(t *testing.T, fp string) *mockSquareProcessor {
	t.Helper()
	m := squareMock("cust_trial", "ccof_trial", "sqpay_trial")
	m.vaultCard = processor.Card{Fingerprint: fp, Brand: "VISA", Last4: "4242", ExpMonth: 12, ExpYear: 2030}
	withFakeSquare(t, m)
	return m
}

// buyTrial buys plan slug with a fresh card as subject through the sale core,
// under its own retry key.
func buyTrial(t *testing.T, ctx context.Context, org *organization.Organization, subject, slug string) *Sale {
	t.Helper()
	sale, err := Subscribe(ctx, org, SubscribeIn{
		SourceID: "cnon:ok", PlanID: slug, Subject: subject, IdempotencyKey: subject + ":" + slug + ":" + strconv.FormatInt(time.Now().UnixNano(), 10),
	})
	if err != nil {
		t.Fatalf("sale of %s to %s: %v", slug, subject, err)
	}
	return sale
}

// endTrial moves a trialing subscription's trial into the past, keeping its
// first paid period right after it, as StartSubscription laid it out.
func endTrial(t *testing.T, db *datastore.Datastore, id string) *subscription.Subscription {
	t.Helper()
	s := reload(t, db, id)
	shift := time.Until(s.TrialEnd) + time.Hour
	s.TrialStart, s.TrialEnd = s.TrialStart.Add(-shift), s.TrialEnd.Add(-shift)
	s.PeriodStart, s.PeriodEnd = s.TrialEnd, engine.Advance(s.TrialEnd, &s.Plan)
	if err := s.Update(); err != nil {
		t.Fatalf("end trial: %v", err)
	}
	return s
}

func TestSubscribeTrial_NoChargeCardVaultedTrialing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("trial-open")
	m := trialSquare(t, "fp_open")
	trialPlan(t, ctx, "uni-pro", trialCents, 7)

	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"uni-pro"}`, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", resp.StatusCode, bodyOf(resp))
	}
	out := jsonBody(t, resp)
	if out["amountCents"].(float64) != 0 || out["invoiceId"] != "" || out["status"] != "ok" {
		t.Fatalf("receipt %+v, want amountCents 0, no invoice, status ok", out)
	}
	if out["trialEndCents"].(float64) != trialCents {
		t.Fatalf("trialEndCents=%v, want %d", out["trialEndCents"], trialCents)
	}
	end, err := time.Parse(time.RFC3339Nano, out["trialEnd"].(string))
	if err != nil {
		t.Fatalf("trialEnd %v: %v", out["trialEnd"], err)
	}

	if m.chargeCalls != 0 {
		t.Fatalf("charge calls=%d, want 0 — a trial charges nothing today", m.chargeCalls)
	}
	if m.addCardNonce != "cnon:ok" {
		t.Fatalf("vaulted nonce=%q, want cnon:ok — the card is validated by vaulting it", m.addCardNonce)
	}
	db := datastore.New(org.Namespaced(ctx))
	pms := pmsFor(t, db, "trial-open")
	if len(pms) != 1 || pms[0].ProviderRef != "ccof_trial" || !pms[0].IsDefault {
		t.Fatalf("payment methods %+v, want the vaulted card, default", pms)
	}

	sub := parentSub(t, db, "trial-open", "uni-pro")
	if sub == nil {
		t.Fatal("no subscription opened")
	}
	if sub.Status != subscription.Trialing {
		t.Fatalf("status=%s, want trialing", sub.Status)
	}
	if want := sub.TrialStart.AddDate(0, 0, 7); !sub.TrialEnd.Equal(want) {
		t.Fatalf("trial ends %s, want %s: seven days after it began", sub.TrialEnd, want)
	}
	if since := time.Since(sub.TrialStart); since < 0 || since > time.Minute {
		t.Fatalf("trial began %s ago, want at the sale", since)
	}
	if !end.Equal(sub.TrialEnd) {
		t.Fatalf("receipt trialEnd %s, subscription %s", end, sub.TrialEnd)
	}
	if sub.DefaultPaymentMethod != pms[0].Id() || sub.ProviderType != string(processor.Square) {
		t.Fatalf("subscription card=%q provider=%q, want %q on square", sub.DefaultPaymentMethod, sub.ProviderType, pms[0].Id())
	}
	if _, err := promo.TakeTrial(ctx, "uni-pro", instrumentOf(pms[0]), false); !errors.Is(err, promo.ErrTrialTaken) {
		t.Fatalf("taking the card's trial again: %v, want ErrTrialTaken — the trial was not recorded against the card", err)
	}
	if invs := invoicesForSub(t, db, sub.Id()); len(invs) != 0 {
		t.Fatalf("invoices=%d, want none during the trial", len(invs))
	}

	// The trial is the subject's subscription: a second paid plan is refused
	// before any card is touched.
	if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:2","planId":"dev"}`, nil); r.StatusCode != http.StatusConflict {
		t.Fatalf("second plan during the trial: status=%d %s, want 409", r.StatusCode, bodyOf(r))
	}
	if m.chargeCalls != 0 {
		t.Fatalf("charge calls=%d after a refused second plan, want 0", m.chargeCalls)
	}
}

// A subject that has had the plan before is sold it exactly as a plan without a
// trial: charged now, active, first period invoiced paid.
func TestSubscribeTrial_SecondTrialForSubjectIsCharged(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("trial-again")
	m := trialSquare(t, "fp_again")
	trialPlan(t, ctx, "uni-pro", trialCents, 7)
	db := datastore.New(org.Namespaced(ctx))

	first := buyTrial(t, ctx, org, "trial-again", "uni-pro")
	if first.TrialEnd == nil {
		t.Fatal("the first purchase opened no trial")
	}
	if _, err := cancelSubscription(ctx, org, AnyHolder, first.SubscriptionID, false); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	// A different card, so only the subject's history can refuse the trial.
	m.vaultCard.Fingerprint = "fp_again_other"
	again := buyTrial(t, ctx, org, "trial-again", "uni-pro")
	if again.TrialEnd != nil || again.TrialEndCents != 0 {
		t.Fatalf("second purchase opened a trial ending %v", again.TrialEnd)
	}
	if again.AmountCents != trialCents || m.chargeCalls != 1 || m.lastChargeAmount != trialCents {
		t.Fatalf("charged %d (calls %d, receipt %d), want once for %d", m.lastChargeAmount, m.chargeCalls, again.AmountCents, trialCents)
	}
	sub := reload(t, db, again.SubscriptionID)
	if sub.Status != subscription.Active || !sub.TrialStart.IsZero() {
		t.Fatalf("status=%s trialStart=%s, want active with no trial", sub.Status, sub.TrialStart)
	}
	invs := invoicesForSub(t, db, sub.Id())
	if len(invs) != 1 || invs[0].Status != billinginvoice.Paid {
		t.Fatalf("invoices=%d, want one paid first invoice", len(invs))
	}
	heldIsPaid(t, sub, invs[0], 0, 1)
}

// One card, one trial of a plan, whoever holds it — and the card is known after
// the first holder removed it.
func TestSubscribeTrial_SameCardNewSubjectIsCharged(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("trial-card")
	m := trialSquare(t, "fp_shared")
	trialPlan(t, ctx, "uni-pro", trialCents, 7)
	db := datastore.New(org.Namespaced(ctx))

	alice := buyTrial(t, ctx, org, "trial-card/alice", "uni-pro")
	if alice.TrialEnd == nil {
		t.Fatal("alice's purchase opened no trial")
	}
	for _, listed := range pmsFor(t, db, "trial-card/alice") {
		pm := paymentmethod.New(db)
		if err := pm.GetById(listed.Id()); err != nil {
			t.Fatalf("load alice's card: %v", err)
		}
		if err := pm.Delete(); err != nil {
			t.Fatalf("remove alice's card: %v", err)
		}
	}
	if n := len(pmsFor(t, db, "trial-card/alice")); n != 0 {
		t.Fatalf("alice still holds %d card(s)", n)
	}

	bob := buyTrial(t, ctx, org, "trial-card/bob", "uni-pro")
	if bob.TrialEnd != nil {
		t.Fatalf("bob opened a second trial on alice's card, ending %v", bob.TrialEnd)
	}
	if bob.AmountCents != trialCents || m.chargeCalls != 1 {
		t.Fatalf("bob charged %d (calls %d), want once for %d", bob.AmountCents, m.chargeCalls, trialCents)
	}
	if s := reload(t, db, bob.SubscriptionID); s.Status != subscription.Active {
		t.Fatalf("bob's subscription %s, want active", s.Status)
	}

	// Another card is another trial.
	m.vaultCard.Fingerprint = "fp_carol"
	if carol := buyTrial(t, ctx, org, "trial-card/carol", "uni-pro"); carol.TrialEnd == nil || carol.AmountCents != 0 {
		t.Fatalf("carol's own card opened no trial: %+v", carol)
	}
	if m.chargeCalls != 1 {
		t.Fatalf("charge calls=%d, want carol's trial uncharged", m.chargeCalls)
	}
}

// Every buyer founds its own org, and each org is its own namespace: the card
// rule spans them all. Two buyer orgs presenting one card get one trial between
// them; the second is charged. A sandbox org's trial is its own offer, so it
// neither spends nor is spent by the live one.
func TestSubscribeTrial_SameCardAcrossBuyerOrgsIsCharged(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	m := trialSquare(t, "fp_orgs")
	trialPlan(t, ctx, "uni-pro", trialCents, 7)

	first := moneyOrg("trial-org-a")
	if sale := buyTrial(t, ctx, first, "trial-org-a", "uni-pro"); sale.TrialEnd == nil || sale.AmountCents != 0 {
		t.Fatalf("the first org's purchase opened no trial: %+v", sale)
	}

	second := moneyOrg("trial-org-b")
	sale := buyTrial(t, ctx, second, "trial-org-b", "uni-pro")
	if sale.TrialEnd != nil {
		t.Fatalf("a second org opened a second trial on the same card, ending %v", sale.TrialEnd)
	}
	if sale.AmountCents != trialCents || m.chargeCalls != 1 || m.lastChargeAmount != trialCents {
		t.Fatalf("second org charged %d (calls %d, receipt %d), want once for %d", m.lastChargeAmount, m.chargeCalls, sale.AmountCents, trialCents)
	}
	db := datastore.New(second.Namespaced(ctx))
	sub := reload(t, db, sale.SubscriptionID)
	if sub.Status != subscription.Active || !sub.TrialStart.IsZero() {
		t.Fatalf("second org's subscription %s (trial from %s), want active with no trial", sub.Status, sub.TrialStart)
	}
	if invs := invoicesForSub(t, db, sub.Id()); len(invs) != 1 || invs[0].Status != billinginvoice.Paid {
		t.Fatalf("second org's invoices %+v, want one paid", invs)
	}

	sandbox := moneyOrg("trial-org-test")
	sandbox.Live = false
	if s := buyTrial(t, ctx, sandbox, "trial-org-test", "uni-pro"); s.TrialEnd == nil || m.chargeCalls != 1 {
		t.Fatalf("a sandbox org on the same card opened no trial: %+v (charges %d)", s, m.chargeCalls)
	}
}

// The trial's end is a renewal: the sweep charges the card once for the first
// paid period, under the per-period key, at the price the receipt stated, and the
// row goes active on that period.
func TestSubscribeTrial_EndChargesOnceAndActivates(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("trial-end")
	m := trialSquare(t, "fp_end")
	trialPlan(t, ctx, "uni-pro", trialCents, 7)
	db := datastore.New(org.Namespaced(ctx))

	sale, err := Subscribe(ctx, org, SubscribeIn{
		SourceID: "cnon:ok", PlanID: "uni-pro", Subject: "trial-end",
		Promo: &promo.Promo{PercentOff: 20, Active: true},
	})
	if err != nil {
		t.Fatalf("sale: %v", err)
	}
	want := engine.Discounted(trialCents, 20)
	if sale.AmountCents != 0 || sale.TrialEndCents != want {
		t.Fatalf("receipt charged %d, states %d at trial end; want 0 and %d", sale.AmountCents, sale.TrialEndCents, want)
	}

	// Before the trial ends nothing is due.
	if out := renewDue(ctx, org, ""); len(out) != 0 || m.chargeCalls != 0 {
		t.Fatalf("sweep during the trial = %+v (charges %d), want nothing", out, m.chargeCalls)
	}

	trialing := endTrial(t, db, sale.SubscriptionID)
	out := renewDue(ctx, org, "")
	if len(out) != 1 || !out[0].Success || out[0].AmountCents != want {
		t.Fatalf("sweep at trial end = %+v, want one success charging %d", out, want)
	}
	if m.chargeCalls != 1 || m.lastChargeAmount != want || m.lastChargeToken != "ccof_trial" || m.lastChargeCustomer != "cust_trial" {
		t.Fatalf("charged %d time(s), %d on %s/%s; want once, %d on the vaulted card", m.chargeCalls, m.lastChargeAmount, m.lastChargeToken, m.lastChargeCustomer, want)
	}
	key := "collect:sub:" + sale.SubscriptionID + ":period:" + strconv.FormatInt(trialing.TrialEnd.Unix(), 10) + ":attempt:0"
	if _, ok := m.chargedKeys[key]; !ok || len(m.chargedKeys) != 1 {
		t.Fatalf("charge keys %v, want exactly %q", m.chargedKeys, key)
	}

	sub := reload(t, db, sale.SubscriptionID)
	if sub.Status != subscription.Active {
		t.Fatalf("status=%s, want active", sub.Status)
	}
	if sub.PeriodStart.Unix() != trialing.TrialEnd.Unix() || sub.PeriodEnd.Unix() != engine.Advance(trialing.TrialEnd, &sub.Plan).Unix() {
		t.Fatalf("row holds %s..%s, want the month from the trial's end %s", sub.PeriodStart, sub.PeriodEnd, trialing.TrialEnd)
	}
	invs := invoicesForSub(t, db, sub.Id())
	if len(invs) != 1 || invs[0].Status != billinginvoice.Paid || invs[0].AmountPaid != want || invs[0].PaymentMethod != "card" {
		t.Fatalf("invoices %+v, want one paid by card for %d", invs, want)
	}
	if sub.CurrentInvoiceId != invs[0].Id() || invs[0].PeriodStart.Unix() != trialing.TrialEnd.Unix() {
		t.Fatalf("row invoice %q period %s, want %q from the trial's end", sub.CurrentInvoiceId, invs[0].PeriodStart, invs[0].Id())
	}

	// The next sweep finds nothing due and charges nothing.
	if out := renewDue(ctx, org, ""); len(out) != 0 || m.chargeCalls != 1 {
		t.Fatalf("second sweep = %+v (charges %d), want nothing", out, m.chargeCalls)
	}
}

// A card declined at the trial's end is a declined renewal: one attempt, the
// invoice left open, the row past due, no second charge from the sweep, and the
// plan back once the invoice is paid.
func TestSubscribeTrial_EndDeclineIsARenewalDecline(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("trial-decline")
	m := trialSquare(t, "fp_decline")
	trialPlan(t, ctx, "uni-pro", trialCents, 7)
	db := datastore.New(org.Namespaced(ctx))

	sale := buyTrial(t, ctx, org, "trial-decline", "uni-pro")
	trialing := endTrial(t, db, sale.SubscriptionID)
	m.chargeErr = errors.New("CARD_DECLINED")

	out := renewDue(ctx, org, "")
	if len(out) != 1 || out[0].Success || out[0].InvoiceId == "" {
		t.Fatalf("sweep = %+v, want one unpaid invoice", out)
	}
	if m.chargeCalls != 1 {
		t.Fatalf("charge calls=%d, want 1", m.chargeCalls)
	}
	sub := reload(t, db, sale.SubscriptionID)
	if sub.Status != subscription.PastDue || sub.PeriodEnd.Unix() != trialing.TrialEnd.Unix() {
		t.Fatalf("row %s holding to %s, want past due owing the period from %s", sub.Status, sub.PeriodEnd, trialing.TrialEnd)
	}
	invs := invoicesForSub(t, db, sub.Id())
	if len(invs) != 1 || invs[0].Status != billinginvoice.Open || invs[0].AttemptCount != 1 {
		t.Fatalf("invoices %+v, want one open after one attempt", invs)
	}

	if out := renewDue(ctx, org, ""); len(out) != 1 || out[0].Success || m.chargeCalls != 1 {
		t.Fatalf("second sweep = %+v (charges %d), want the same open invoice and no charge", out, m.chargeCalls)
	}
	if n := len(invoicesForSub(t, db, sub.Id())); n != 1 {
		t.Fatalf("invoices=%d after the second sweep, want 1", n)
	}

	m.chargeErr = nil
	paid, f := CollectInvoice(ctx, org, invs[0].Id(), nil, nil)
	if f != nil || !paid.Paid || paid.CardChargedCents != trialCents {
		t.Fatalf("pay the invoice: %+v, %v", paid, f)
	}
	if out := renewDue(ctx, org, ""); len(out) != 1 || !out[0].Success {
		t.Fatalf("sweep after paying = %+v, want the paid period read back", out)
	}
	if s := reload(t, db, sale.SubscriptionID); s.Status != subscription.Active {
		t.Fatalf("status=%s after paying, want active", s.Status)
	}
	if m.chargeCalls != 2 {
		t.Fatalf("charge calls=%d, want the decline and the payment", m.chargeCalls)
	}
}

// Cancelling during the trial ends it with no charge, now or at its end.
func TestSubscribeTrial_CancelDuringTrialNeverCharges(t *testing.T) {
	for _, atEnd := range []bool{false, true} {
		t.Run("atPeriodEnd="+strconv.FormatBool(atEnd), func(t *testing.T) {
			ctx := ae.NewContext()
			defer ctx.Close()
			org := moneyOrg("trial-cancel")
			m := trialSquare(t, "fp_cancel")
			trialPlan(t, ctx, "uni-pro", trialCents, 7)
			db := datastore.New(org.Namespaced(ctx))

			sale := buyTrial(t, ctx, org, "trial-cancel", "uni-pro")
			if _, err := cancelSubscription(ctx, org, AnyHolder, sale.SubscriptionID, atEnd); err != nil {
				t.Fatalf("cancel: %v", err)
			}
			trialing := endTrial(t, db, sale.SubscriptionID)
			for i := 0; i < 2; i++ {
				renewDue(ctx, org, "")
			}

			if m.chargeCalls != 0 {
				t.Fatalf("charge calls=%d, want 0 — a canceled trial is never charged", m.chargeCalls)
			}
			sub := reload(t, db, sale.SubscriptionID)
			if sub.Status != subscription.Canceled {
				t.Fatalf("status=%s, want canceled", sub.Status)
			}
			if atEnd && sub.Ended.Unix() != trialing.TrialEnd.Unix() {
				t.Fatalf("ended %s, want at the trial's end %s", sub.Ended, trialing.TrialEnd)
			}
			if invs := invoicesForSub(t, db, sub.Id()); len(invs) != 0 {
				t.Fatalf("invoices=%d, want none", len(invs))
			}
		})
	}
}

// A plan with no trial sells exactly as before: charged now, active, first
// period paid, and the receipt says nothing about a trial.
func TestSubscribeTrial_NoTrialPlanUnchanged(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("trial-none")
	m := trialSquare(t, "fp_none")
	trialPlan(t, ctx, "uni-basic", trialCents, 0)

	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"uni-basic"}`, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", resp.StatusCode, bodyOf(resp))
	}
	out := jsonBody(t, resp)
	if _, ok := out["trialEnd"]; ok {
		t.Fatalf("receipt %+v names a trial", out)
	}
	if _, ok := out["trialEndCents"]; ok {
		t.Fatalf("receipt %+v names a trial charge", out)
	}
	if out["amountCents"].(float64) != trialCents || m.chargeCalls != 1 || m.lastChargeAmount != trialCents {
		t.Fatalf("charged %d (calls %d), receipt %v; want once for %d", m.lastChargeAmount, m.chargeCalls, out["amountCents"], trialCents)
	}
	db := datastore.New(org.Namespaced(ctx))
	sub := parentSub(t, db, "trial-none", "uni-basic")
	if sub == nil || sub.Status != subscription.Active || !sub.TrialStart.IsZero() {
		t.Fatalf("subscription %+v, want active with no trial", sub)
	}
	release, err := promo.TakeTrial(ctx, "uni-basic", instrumentOf(pmsFor(t, db, "trial-none")[0]), false)
	if err != nil {
		t.Fatalf("a paid sale spent the card's trial: %v", err)
	}
	release()
	invs := invoicesForSub(t, db, sub.Id())
	if len(invs) != 1 || invs[0].Status != billinginvoice.Paid {
		t.Fatalf("invoices=%d, want one paid first invoice", len(invs))
	}
	heldIsPaid(t, sub, invs[0], 0, 1)
}

// A coupon is a percent off a charge made today, and a trial makes none: a sale
// that would open a trial refuses the coupon before it is reserved or anything is
// charged, and leaves no card behind.
func TestSubscribeTrial_CouponRefused(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	trialPlan(t, ctx, "uni-pro", trialCents, 7)
	warmCouponNS(t, ctx)
	now := time.Now().UTC()
	start, end := now.Add(-time.Minute), now.Add(24*time.Hour)
	planCoupon(t, ctx, "UNI50", promo.CouponSpec{
		Percent: 50, Start: &start, End: &end, Limit: 5, Plans: []string{"uni-pro"}, Enabled: true,
	})
	org := moneyOrg("trial-coupon")
	m := trialSquare(t, "fp_coupon")

	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"uni-pro","coupon":"UNI50"}`, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400", resp.StatusCode, bodyOf(resp))
	}
	if m.chargeCalls != 0 || !m.removeCalled {
		t.Fatalf("charge calls=%d, card removed=%v; want no charge and the fresh card removed", m.chargeCalls, m.removeCalled)
	}
	db := datastore.New(org.Namespaced(ctx))
	if sub := parentSub(t, db, "trial-coupon", "uni-pro"); sub != nil {
		t.Fatalf("a refused sale opened subscription %s", sub.Id())
	}
	if pms := pmsFor(t, db, "trial-coupon"); len(pms) != 0 {
		t.Fatalf("payment methods=%d, want the fresh card removed", len(pms))
	}
	if n := couponUses(t, ctx, "UNI50"); n != 0 {
		t.Fatalf("coupon used %d, want 0", n)
	}

	// Without the coupon the same buyer starts the trial.
	if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"uni-pro"}`, nil); r.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", r.StatusCode, bodyOf(r))
	}
	if sub := parentSub(t, db, "trial-coupon", "uni-pro"); sub == nil || sub.Status != subscription.Trialing {
		t.Fatalf("subscription %+v, want trialing", sub)
	}
}

// The operator is told a trial began, what it costs when it ends, and when.
func TestSubscribeTrial_SaleNoticeSaysTrial(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	trialSquare(t, "fp_notice")
	trialPlan(t, ctx, "uni-pro", trialCents, 7)

	got := make(chan sent, 2)
	mail.Set(senderFunc(func(_ context.Context, to []string, subject, body string) error {
		got <- sent{to, subject, body}
		return nil
	}))
	t.Cleanup(func() { mail.Set(nil) })
	t.Setenv(saleNotifyEnv, "ops@example.test")

	sale := buyTrial(t, ctx, moneyOrg("trial-notice"), "trial-notice", "uni-pro")
	var s sent
	select {
	case s = <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("no notice for a trial")
	}
	day := sale.TrialEnd.UTC().Format(time.DateOnly)
	if !strings.HasPrefix(s.subject, "Trial: uni-pro (month) for trial-notice, 29.00 USD from "+day) {
		t.Fatalf("notice subject %q", s.subject)
	}
	for _, want := range []string{
		"A free trial of a paid plan was started.",
		"Charged:        0.00 USD",
		"Trial ends:     " + sale.TrialEnd.UTC().Format(time.RFC3339),
		"Then charged:   29.00 USD",
	} {
		if !strings.Contains(s.body, want) {
			t.Errorf("notice body lacks %q:\n%s", want, s.body)
		}
	}
}
