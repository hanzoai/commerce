package billing

import (
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/tier"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A gift is days of a plan given, not sold. These pin what it gives, that it
// gives once per key, and that a gift to a Free subject ends on its own.

const week = 7

func giftRows(t *testing.T, db *datastore.Datastore, subject string) []*subscription.Subscription {
	t.Helper()
	var out []*subscription.Subscription
	for _, s := range subsOf(t, db, subject) {
		if s.ProviderType == giftProvider {
			out = append(out, s)
		}
	}
	return out
}

// buyDev subscribes subject to Pro with a card, as checkout does, and answers the
// row.
func buyDev(t *testing.T, org *organization.Organization, ctx ae.Context, subject string) *subscription.Subscription {
	t.Helper()
	sale, err := Subscribe(ctx, org, SubscribeIn{SourceID: "cnon:ok", PlanID: "dev", Subject: subject})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	db := datastore.New(org.Namespaced(ctx))
	sub := subscription.New(db)
	if err := sub.GetById(sale.SubscriptionID); err != nil {
		t.Fatalf("read subscription: %v", err)
	}
	return sub
}

func TestGift_FreeSubjectGetsProForTheDaysThenFree(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("gift-free")
	db := datastore.New(org.Namespaced(ctx))

	got, err := Gift(ctx, org, GiftIn{Subject: "hanzo/ann", Days: week, Key: "referral:bob", Reason: "referral"})
	if err != nil {
		t.Fatalf("gift: %v", err)
	}
	if got.Outcome != GiftOpened {
		t.Fatalf("outcome=%q, want %q", got.Outcome, GiftOpened)
	}
	rows := giftRows(t, db, "hanzo/ann")
	if len(rows) != 1 {
		t.Fatalf("gift rows=%d, want 1", len(rows))
	}
	s := rows[0]
	if s.Plan.Slug != "dev" && s.PlanId != "dev" {
		t.Fatalf("gift plan=%q/%q, want dev (Pro)", s.Plan.Slug, s.PlanId)
	}
	if !s.EndCancel || s.Status != subscription.Active {
		t.Fatalf("status=%q endCancel=%v, want active and canceled at the end of its days", s.Status, s.EndCancel)
	}
	if d := s.PeriodEnd.Sub(s.PeriodStart); d != week*24*time.Hour {
		t.Fatalf("period=%v, want exactly %d days", d, week)
	}
	if tr, _ := deriveTier(db, "hanzo/ann", false); tr != tier.Pro {
		t.Fatalf("tier during the gift=%q, want pro", tr)
	}
	if billingSubscription(db, "hanzo/ann", false) != nil {
		t.Fatal("a gift is not a paid plan: it must never block buying one")
	}

	// The days pass. No sweep runs: the row is still Active in the store, and it
	// confers nothing.
	row := subscription.New(db)
	if err := row.GetById(s.Id()); err != nil {
		t.Fatal(err)
	}
	row.PeriodStart, row.PeriodEnd = row.PeriodStart.Add(-8*24*time.Hour), row.PeriodEnd.Add(-8*24*time.Hour)
	if err := row.Update(); err != nil {
		t.Fatal(err)
	}
	if tr, _ := deriveTier(db, "hanzo/ann", false); tr != tier.Free {
		t.Fatalf("tier after the gift=%q, want free", tr)
	}
	if _, err := ReactivateSubscription(ctx, org, AnyHolder, s.Id()); err == nil || !IsSubscriptionRefused(err) {
		t.Fatalf("reactivating a gift answered %v, want a refusal", err)
	}
}

func TestGift_PaidSubjectsPeriodRunsTheDaysLonger(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("gift-paid")
	withFakeSquare(t, squareMock("cust_g", "ccof_g", "sqpay_g"))
	db := datastore.New(org.Namespaced(ctx))
	sub := buyDev(t, org, ctx, "gift-paid")
	end := sub.PeriodEnd

	got, err := Gift(ctx, org, GiftIn{Subject: "gift-paid", Days: week, Key: "referral:bob"})
	if err != nil {
		t.Fatalf("gift: %v", err)
	}
	if got.Outcome != GiftExtended {
		t.Fatalf("outcome=%q, want %q", got.Outcome, GiftExtended)
	}
	after := subscription.New(db)
	if err := after.GetById(sub.Id()); err != nil {
		t.Fatal(err)
	}
	if want := end.Add(week * 24 * time.Hour); !after.PeriodEnd.Equal(want) {
		t.Fatalf("period end=%v, want %v", after.PeriodEnd, want)
	}
	if len(giftRows(t, db, "gift-paid")) != 0 {
		t.Fatal("a paid subject's gift opened a second row")
	}
	if n := len(invoicesForSub(t, db, sub.Id())); n != 1 {
		t.Fatalf("invoices=%d, want the one the sale paid: a gift raises none", n)
	}
}

func TestGift_AKeyGivesOnceAndTwoKeysStack(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("gift-once")
	db := datastore.New(org.Namespaced(ctx))

	for range 3 {
		if _, err := Gift(ctx, org, GiftIn{Subject: "hanzo/cat", Days: week, Key: "referral:one"}); err != nil {
			t.Fatalf("gift: %v", err)
		}
	}
	rows := giftRows(t, db, "hanzo/cat")
	if len(rows) != 1 || rows[0].PeriodEnd.Sub(rows[0].PeriodStart) != week*24*time.Hour {
		t.Fatalf("one key given three times: rows=%d; want one row of %d days", len(rows), week)
	}

	// The same key again after its guard is gone still gives nothing: the row
	// that received it remembers.
	if _, err := Gift(ctx, org, GiftIn{Subject: "hanzo/cat", Days: week, Key: "referral:two"}); err != nil {
		t.Fatalf("second gift: %v", err)
	}
	rows = giftRows(t, db, "hanzo/cat")
	if len(rows) != 1 || rows[0].PeriodEnd.Sub(rows[0].PeriodStart) != 2*week*24*time.Hour {
		t.Fatalf("two keys: rows=%d; want one row of %d days", len(rows), 2*week)
	}
	if !gave(rows[0], "referral:one") || !gave(rows[0], "referral:two") {
		t.Fatal("the row does not remember both gifts")
	}
	before := rows[0].PeriodEnd
	out, err := give(ctx, org, db, "hanzo/cat", "referral:one", GiftIn{Subject: "hanzo/cat", Days: week}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Subscription.CurrentPeriodEnd.Equal(before) {
		t.Fatalf("a remembered key gave again: end %v, want %v", out.Subscription.CurrentPeriodEnd, before)
	}
}

func TestGift_SandboxTimeNeverLandsOnALiveRow(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("gift-test")
	withFakeSquare(t, squareMock("cust_t", "ccof_t", "sqpay_t"))
	db := datastore.New(org.Namespaced(ctx))
	sub := buyDev(t, org, ctx, "gift-test")

	if _, err := Gift(ctx, org, GiftIn{Subject: "gift-test", Days: week, Key: "referral:sandbox", Test: true}); err != nil {
		t.Fatalf("gift: %v", err)
	}
	live := subscription.New(db)
	if err := live.GetById(sub.Id()); err != nil {
		t.Fatal(err)
	}
	if !live.PeriodEnd.Equal(sub.PeriodEnd) {
		t.Fatal("a sandbox gift moved the live plan")
	}
	rows := giftRows(t, db, "gift-test")
	if len(rows) != 1 || !rows[0].Test {
		t.Fatalf("sandbox gift rows=%d, want one test row", len(rows))
	}
}

func TestGift_Refuses(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("gift-refuse")
	for _, in := range []GiftIn{
		{Days: week, Key: "k"},
		{Subject: "s", Days: week},
		{Subject: "s", Days: 0, Key: "k"},
		{Subject: "s", Days: giftMost + 1, Key: "k"},
	} {
		if _, err := Gift(ctx, org, in); err == nil || !IsSaleRefused(err) {
			t.Fatalf("%+v answered %v, want a refusal", in, err)
		}
	}
}

func TestPaymentsOf_FirstCardPeriodAndCards(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("pay-first")
	m := squareMock("cust_p", "ccof_p", "sqpay_p")
	m.vaultCard = processor.Card{Brand: "Visa", Last4: "4242", ExpMonth: 12, ExpYear: 2030, Fingerprint: "fp-card-1"}
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))

	none, err := PaymentsOf(ctx, org, "pay-first")
	if err != nil || none.First != nil || len(none.Cards) != 0 {
		t.Fatalf("before any sale: %+v %v, want nothing", none, err)
	}

	sub := buyDev(t, org, ctx, "pay-first")
	got, err := PaymentsOf(ctx, org, "pay-first")
	if err != nil {
		t.Fatalf("payments: %v", err)
	}
	if got.First == nil || got.First.Subscription != sub.Id() || got.First.Cents <= 0 || got.First.Card != "fp-card-1" {
		t.Fatalf("first=%+v, want the dev sale paid by fp-card-1", got.First)
	}
	if got.First.Refunded != 0 || got.First.Disputed || got.First.Test {
		t.Fatalf("first=%+v, want a live payment, not refunded, not disputed", got.First)
	}
	if len(got.Cards) != 1 || got.Cards[0] != "fp-card-1" {
		t.Fatalf("cards=%v, want [fp-card-1]", got.Cards)
	}

	// A refund against it is reported, and so is a chargeback.
	inv := invoicesForSub(t, db, sub.Id())[0]
	row := billinginvoice.New(db)
	if err := row.GetById(inv.Id()); err != nil {
		t.Fatal(err)
	}
	row.Refund("rf_1", 500)
	row.SetDispute("EVIDENCE_REQUIRED", 1, time.Now())
	if err := row.Update(); err != nil {
		t.Fatal(err)
	}
	got, _ = PaymentsOf(ctx, org, "pay-first")
	if got.First.Refunded != 500 || !got.First.Disputed {
		t.Fatalf("first=%+v, want refunded 500 and disputed", got.First)
	}
}

func TestPaymentsOf_PrepaidIsNotACard(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("pay-credit")
	db := datastore.New(org.Namespaced(ctx))
	inv := billinginvoice.New(db)
	inv.UserId = "pay-credit"
	inv.SubscriptionId = "sub_x"
	inv.Status = billinginvoice.Paid
	inv.PaymentMethod = "credit"
	inv.AmountPaid = 2000
	inv.PaidAt = time.Now()
	if err := inv.Create(); err != nil {
		t.Fatal(err)
	}
	got, err := PaymentsOf(ctx, org, "pay-credit")
	if err != nil || got.First != nil {
		t.Fatalf("a period paid from credit read as %+v (%v), want no card payment", got.First, err)
	}
}
