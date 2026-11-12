package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/billing/tier"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/creditgrant"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A plan is billed to a card: never bought or renewed from prepaid money, which
// pays for usage. These pin every way that used to happen, the card that renews
// in its place, the plan collected outside commerce that lapses at its end, the
// card plan that starts there, the reminders before it, a customer's own price,
// and staff moving a period's end.

const (
	balSlug    = "retainer" // a private plan: priced by agreement, not in the catalog
	balPrice   = 2500
	balSubject = "acme"
)

// balanceSetup seeds the catalog, writes the private plan, and returns an org
// whose subject holds funded cents on commerce's own ledger.
func balanceSetup(t *testing.T, ctx context.Context, orgName string, funded int64) (*organization.Organization, *datastore.Datastore) {
	t.Helper()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	authorityPlan(t, ctx, balSlug, balSlug, plan.StatusPrivate, balPrice)
	org := moneyOrg(orgName)
	db := datastore.New(org.Namespaced(ctx))
	if funded > 0 {
		deposit(t, db, balSubject, funded)
	}
	return org, db
}

// deposit credits the subject's balance the way a wire recorded as prepaid
// credit lands on commerce's own ledger.
func deposit(t *testing.T, db *datastore.Datastore, subject string, cents int64) {
	t.Helper()
	tx := transaction.New(db)
	tx.Type = transaction.Deposit
	tx.DestinationKind = transaction.IAMUserKind
	tx.DestinationId = subject
	tx.Currency = currency.USD
	tx.Amount = currency.Cents(cents)
	tx.Tags = "prepaid"
	if err := tx.Create(); err != nil {
		t.Fatalf("deposit: %v", err)
	}
}

func grant(t *testing.T, db *datastore.Datastore, subject string, cents int64) *creditgrant.CreditGrant {
	t.Helper()
	g := creditgrant.New(db)
	g.UserId, g.Name = subject, "comp"
	g.AmountCents, g.RemainingCents = cents, cents
	g.Currency = currency.USD
	g.EffectiveAt = time.Now().Add(-time.Hour)
	if err := g.Create(); err != nil {
		t.Fatalf("grant: %v", err)
	}
	return g
}

func walletOf(t *testing.T, ctx context.Context, org *organization.Organization, subject string) int64 {
	t.Helper()
	b, err := prepaidFor(ctx, org).balance(ctx, subject, currency.USD)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	return b
}

// due moves the row's current period into the past, so the cycle owes it.
func due(t *testing.T, db *datastore.Datastore, id string) {
	t.Helper()
	s := subscription.New(db)
	if err := s.GetById(id); err != nil {
		t.Fatalf("load: %v", err)
	}
	s.PeriodStart, s.PeriodEnd = time.Now().AddDate(0, -1, -1), time.Now().Add(-time.Hour)
	if err := s.Update(); err != nil {
		t.Fatalf("save: %v", err)
	}
}

func reload(t *testing.T, db *datastore.Datastore, id string) *subscription.Subscription {
	t.Helper()
	s := subscription.New(db)
	if err := s.GetById(id); err != nil {
		t.Fatalf("reload: %v", err)
	}
	return s
}

// lossy is a ledger whose next debit lands and then reports failure — a posting
// whose answer was lost on the way back.
type lossy struct {
	*fakeLedger
	lose bool
}

func (l *lossy) Debit(ctx context.Context, in creditledger.DebitInput) (string, int64, error) {
	id, bal, err := l.fakeLedger.Debit(ctx, in)
	if err == nil && l.lose {
		l.lose = false
		return "", 0, errors.New("the posting was not acknowledged")
	}
	return id, bal, err
}

// refusedAs fails unless err is a sale refusal carrying code.
func refusedAs(t *testing.T, what string, err error, code string) {
	t.Helper()
	var r *Refusal
	if !errors.As(err, &r) || r.Code != code {
		t.Fatalf("%s: err = %v, want refused %q", what, err, code)
	}
}

// legacyBalanceRow stores a plan the balance used to renew, as the balance sale
// and the balance record opened one: provider "credit", paid by "credits", its
// first period paid from the balance. It is due.
func legacyBalanceRow(t *testing.T, ctx context.Context, db *datastore.Datastore, subject, slug string) *subscription.Subscription {
	t.Helper()
	p, err := resolveSubscriptionPlan(db, slug)
	if err != nil {
		t.Fatalf("plan %q: %v", slug, err)
	}
	s := subscription.New(db)
	s.UserId, s.DefaultPaymentMethod, s.ProviderType, s.Quantity = subject, "credits", "credit", 1
	engine.StartSubscription(s, p)
	if err := s.Create(); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := engine.CreatePaidFirstInvoice(db, s, "balance", "led_legacy"); err != nil {
		t.Fatalf("first invoice: %v", err)
	}
	if err := s.Update(); err != nil {
		t.Fatalf("save: %v", err)
	}
	due(t, db, s.Id())
	return reload(t, db, s.Id())
}

// recordIn is a payment recorded as collected outside checkout: a wire for slug
// at cents, over [start, end).
func recordIn(subject, slug string, cents int64, start, end time.Time) RecordIn {
	return RecordIn{
		Subject: subject, PlanID: slug, PriceCents: cents, PeriodStart: start, PeriodEnd: end,
		Processor: "wire", Reference: map[string]string{"payment": "FED-" + slug}, Terms: "paid by wire",
	}
}

// compIn is a plan given away for [start, end).
func compIn(subject, slug string, start, end time.Time) RecordIn {
	return RecordIn{
		Subject: subject, PlanID: slug, PeriodStart: start, PeriodEnd: end,
		Processor: "comp", Reference: map[string]string{"order": "comp-" + subject}, Terms: "given by the owner",
	}
}

// Staff cannot record a plan as paid from prepaid money, under any spelling, and
// nothing is written or drawn. Mutation: drop the prepaidSource case in
// RecordSubscription and "balance" is answered processor_unknown, not plan_needs_card.
func TestRecordSubscription_PrepaidMoneyIsNoProcessor(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := balanceSetup(t, ctx, "pc-record", 20000)
	start := time.Now().UTC().Truncate(24 * time.Hour)
	for _, proc := range []string{"balance", "Balance", "credit", "credits", "prepaid"} {
		in := recordIn(balSubject, balSlug, balPrice, start, start.AddDate(0, 1, 0))
		in.Processor = proc
		_, err := RecordSubscription(ctx, org, in)
		refusedAs(t, "processor "+proc, err, "plan_needs_card")
	}
	in := recordIn(balSubject, balSlug, balPrice, start, start.AddDate(0, 1, 0))
	in.Processor = "venmo"
	_, err := RecordSubscription(ctx, org, in)
	refusedAs(t, "processor venmo", err, "processor_unknown")
	if n := len(subsOf(t, db, balSubject)); n != 0 {
		t.Fatalf("refused records wrote %d subscription(s)", n)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 20000 {
		t.Fatalf("balance = %d, want the 20000 untouched", b)
	}
	for _, proc := range RecordProcessors[:len(RecordProcessors)-1] {
		org, _ := balanceSetup(t, ctx, "pc-record-"+proc, 0)
		in := recordIn(balSubject, balSlug, balPrice, start, start.AddDate(0, 1, 0))
		in.Processor = proc
		if _, err := RecordSubscription(ctx, org, in); err != nil {
			t.Fatalf("processor %s is listed and was refused: %v", proc, err)
		}
	}
}

// A plan is not bought from prepaid money at checkout, whatever the balance holds.
// Mutation: drop the prepaidSource refusal in subscribe and the "balance" source
// is answered as a missing card, not plan_needs_card.
func TestSubscribe_PrepaidMoneyBuysNoPlan(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := moneyOrg("pc-sale")
	db := datastore.New(org.Namespaced(ctx))
	deposit(t, db, org.Name, 100000)
	m := squareMock("cust_pc", "ccof_pc", "sqpay_pc")
	withFakeSquare(t, m)
	for _, src := range []string{"balance", "credits", "credit", "prepaid"} {
		_, err := Subscribe(ctx, org, SubscribeIn{SourceID: src, PlanID: "dev", Subject: org.Name})
		refusedAs(t, "source "+src, err, "plan_needs_card")
	}
	r := invokeSubscribeCard(org, ctx, `{"sourceId":"balance","planId":"dev"}`, nil)
	if r.StatusCode != 400 {
		t.Fatalf("POST subscribe/card from the balance: %d %s, want 400", r.StatusCode, bodyOf(r))
	}
	if n := len(subsOf(t, db, org.Name)); n != 0 || m.chargeCalls != 0 {
		t.Fatalf("refused sales wrote %d subscription(s) and charged %d time(s)", n, m.chargeCalls)
	}
	if b := walletOf(t, ctx, org, org.Name); b != 100000 {
		t.Fatalf("balance = %d, want the 100000 untouched", b)
	}
}

// Card subscriptions are the automatic path: sold by card and renewed on the
// same card, the balance beside it untouched. Mutation: let CollectInvoice draw
// prepaid money for a plan's invoice and the renewal is paid from the balance.
func TestRenewDue_ACardPlanRenewsOnItsCardNotTheBalance(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := moneyOrg("pc-renew")
	db := datastore.New(org.Namespaced(ctx))
	deposit(t, db, org.Name, 100000)
	m := squareMock("cust_rn", "ccof_rn", "sqpay_rn")
	withFakeSquare(t, m)
	price := lookupPlan("go").Price

	if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"go"}`, nil); r.StatusCode != 201 {
		t.Fatalf("sale: %d %s", r.StatusCode, bodyOf(r))
	}
	sub := parentSub(t, db, org.Name, "go")
	if m.chargeCalls != 1 || m.lastChargeAmount != price || sub == nil {
		t.Fatalf("sale charged %d time(s), last %d, row %v; want once at %d", m.chargeCalls, m.lastChargeAmount, sub, price)
	}
	due(t, db, sub.Id())
	out := renewDue(ctx, org, "")
	if len(out) != 1 || !out[0].Success || out[0].AmountCents != price {
		t.Fatalf("sweep = %+v, want the row renewed on its card for %d", out, price)
	}
	if m.chargeCalls != 2 || m.lastChargeAmount != price || m.lastChargeToken != "ccof_rn" {
		t.Fatalf("card charged %d time(s), last %d on %q; want the renewal on the vaulted card", m.chargeCalls, m.lastChargeAmount, m.lastChargeToken)
	}
	if b := walletOf(t, ctx, org, org.Name); b != 100000 {
		t.Fatalf("balance = %d, want the 100000 untouched by a plan", b)
	}
	if s := reload(t, db, sub.Id()); s.Status != subscription.Active {
		t.Fatalf("status = %s, want active", s.Status)
	}
}

// A plan the balance used to renew is renewed from it no longer: its renewal is
// charged to a card, it holds none, and it goes past due with the balance whole.
func TestRenewDue_ALegacyBalancePlanIsNotRenewedFromTheBalance(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := balanceSetup(t, ctx, "pc-legacy", 20000)
	g := grant(t, db, balSubject, 5000)
	s := legacyBalanceRow(t, ctx, db, balSubject, balSlug)

	out := renewDue(ctx, org, "")
	if len(out) != 1 || out[0].Success || out[0].InvoiceId == "" {
		t.Fatalf("sweep = %+v, want the renewal unpaid", out)
	}
	if r := reload(t, db, s.Id()); r.Status != subscription.PastDue {
		t.Fatalf("status = %s, want past_due", r.Status)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 20000 {
		t.Fatalf("balance = %d, want the 20000 untouched", b)
	}
	left := creditgrant.New(db)
	if err := left.GetById(g.Id()); err != nil || left.RemainingCents != 5000 {
		t.Fatalf("credit grant holds %d (err %v), want 5000 untouched", left.RemainingCents, err)
	}
	if tr, _ := deriveTier(db, balSubject, org.TestMode()); tr != tier.Free {
		t.Fatalf("tier = %q, want free while unpaid", tr)
	}
	// Paying the invoice by hand draws nothing from prepaid money either.
	paid, f := CollectInvoice(ctx, org, out[0].InvoiceId, nil)
	if f != nil || paid.Paid || paid.BalanceUsedCents != 0 || paid.CreditUsedCents != 0 {
		t.Fatalf("collect = %+v, %v; want unpaid with no prepaid money used", paid, f)
	}
}

// A plan collected outside commerce is served to the end of its period and ends
// there; the sweep marks it. Mutation: drop External from engine.Over and the row
// past its end is still the subject's plan.
func TestRenewDue_AnExternalPlanLapsesAtItsEnd(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := balanceSetup(t, ctx, "pc-lapse", 0)
	end := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	got, err := RecordSubscription(ctx, org, recordIn(balSubject, balSlug, balPrice, end.AddDate(0, -1, 0), end))
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if held := billingSubscription(db, balSubject, org.TestMode()); held != nil {
		t.Fatalf("a plan past its end is still held: %s", held.Id())
	}
	out := renewDue(ctx, org, "")
	if len(out) != 1 || !out[0].Lapsed || out[0].Success {
		t.Fatalf("sweep = %+v, want the row lapsed", out)
	}
	s := reload(t, db, got.Subscription.ID)
	if s.Status != subscription.Canceled || !s.Ended.Equal(end) {
		t.Fatalf("row %s ended %s, want canceled as of %s", s.Status, s.Ended, end)
	}
	if out := renewDue(ctx, org, ""); len(out) != 0 {
		t.Fatalf("a lapsed row was swept again: %+v", out)
	}

	live, ldb := balanceSetup(t, ctx, "pc-lapse-live", 0)
	start := time.Now().UTC().Truncate(24 * time.Hour)
	running, err := RecordSubscription(ctx, live, recordIn(balSubject, balSlug, balPrice, start, start.AddDate(0, 1, 0)))
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if out := renewDue(ctx, live, ""); len(out) != 0 {
		t.Fatalf("a running external plan was swept: %+v", out)
	}
	if s := reload(t, ldb, running.Subscription.ID); s.Status != subscription.Active {
		t.Fatalf("status = %s, want active until its end", s.Status)
	}
}

// Bought by card while a plan given away runs, the card plan starts where that one
// ends: nothing is charged today, the card is charged once at that end, and the two
// never overlap. Mutation: drop the starts branch in subscribe and the card is
// charged today, over a period the comp already covers.
func TestSubscribe_ACardPlanStartsWhenTheExternalPlanEnds(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := moneyOrg("pc-follow")
	db := datastore.New(org.Namespaced(ctx))
	m := squareMock("cust_fo", "ccof_fo", "sqpay_fo")
	withFakeSquare(t, m)
	start := time.Now().UTC().Truncate(time.Second).AddDate(0, 0, -20)
	end := start.AddDate(0, 0, 30)
	comp, err := RecordSubscription(ctx, org, compIn(org.Name, "go", start, end))
	if err != nil {
		t.Fatalf("comp: %v", err)
	}

	// A wallet keeps no card to charge at the start.
	if r := invokeSubscribeCard(org, ctx, `{"sourceId":"wnon:ok","planId":"go","wallet":true}`, nil); r.StatusCode != 400 {
		t.Fatalf("a wallet for a plan that starts later: %d %s, want 400", r.StatusCode, bodyOf(r))
	}
	r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"go"}`, nil)
	if r.StatusCode != 201 {
		t.Fatalf("sale: %d %s", r.StatusCode, bodyOf(r))
	}
	body := jsonBody(t, r)
	if m.chargeCalls != 0 || m.authorizeCalls != 0 {
		t.Fatalf("charged %d and authorized %d time(s) today; the comp covers today", m.chargeCalls, m.authorizeCalls)
	}
	if body["startsAt"] == nil || body["follows"] != comp.Subscription.ID || body["amountCents"].(float64) != 0 {
		t.Fatalf("receipt %v, want it to start at the comp's end, follow it, and charge nothing", body)
	}
	next := scheduledFor(db, org.Name, false)
	if next == nil || !next.PeriodStart.Equal(end) || next.DefaultPaymentMethod == "" {
		t.Fatalf("scheduled row %+v, want one starting %s on the card", next, end)
	}
	if c := reload(t, db, comp.Subscription.ID); !c.EndCancel || c.Status != subscription.Active {
		t.Fatalf("comp endCancel=%v status=%s, want it active and set to end", c.EndCancel, c.Status)
	}
	if hold := billingSubscription(db, org.Name, false); hold == nil || hold.Id() != comp.Subscription.ID {
		t.Fatalf("held = %v, want the comp until its end", hold)
	}
	if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"go"}`, map[string]string{"X-Idempotency-Key": "second"}); r.StatusCode != 409 {
		t.Fatalf("a second card plan: %d %s, want 409", r.StatusCode, bodyOf(r))
	}

	// The comp's end comes.
	c := reload(t, db, comp.Subscription.ID)
	gone := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	c.PeriodEnd = gone
	if err := c.Update(); err != nil {
		t.Fatalf("save: %v", err)
	}
	n := reload(t, db, next.Id())
	n.Start, n.PeriodStart, n.PeriodEnd = gone, gone, gone
	if err := n.Update(); err != nil {
		t.Fatalf("save: %v", err)
	}
	out := renewDue(ctx, org, "")
	lapsed, renewed := 0, 0
	for _, o := range out {
		switch {
		case o.Lapsed && o.SubscriptionId == comp.Subscription.ID:
			lapsed++
		case o.Success && o.SubscriptionId == next.Id():
			renewed++
		}
	}
	if lapsed != 1 || renewed != 1 {
		t.Fatalf("sweep = %+v, want the comp ended and the card plan's first period charged", out)
	}
	if m.chargeCalls != 1 || m.lastChargeAmount != lookupPlan("go").Price {
		t.Fatalf("card charged %d time(s), last %d; want once, the plan's price", m.chargeCalls, m.lastChargeAmount)
	}
	s := reload(t, db, next.Id())
	if s.Status != subscription.Active || !s.PeriodStart.Equal(gone) {
		t.Fatalf("card plan %s from %s, want active from %s", s.Status, s.PeriodStart, gone)
	}
}

// Every plan that ends without a charge is reminded a week and a day before its
// end, once each, and a failed send is tried again. A card plan, and a plan a card
// plan already follows, are not reminded. Mutation: drop the replay case in
// remindOrg and the week's reminder is sent on every pass.
func TestRemindDue_AWeekThenADayOnceEach(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := moneyOrg("pc-remind")
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now().UTC().Truncate(time.Second)
	end := now.Add(6 * 24 * time.Hour)
	wire, err := RecordSubscription(ctx, org, recordIn("pc-remind/wire", "go", lookupPlan("go").Price, end.AddDate(0, -1, 0), end))
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	legacy := legacyBalanceRow(t, ctx, db, "pc-remind/legacy", "go")
	legacy.PeriodStart, legacy.PeriodEnd = end.AddDate(0, -1, 0), end
	if err := legacy.Update(); err != nil {
		t.Fatalf("save: %v", err)
	}
	card := seedCardBackedSub(t, db, "pc-remind/card", "go", "ccof_x", "cust_x")
	card.PeriodStart, card.PeriodEnd = end.AddDate(0, -1, 0), end
	if err := card.Update(); err != nil {
		t.Fatalf("save: %v", err)
	}
	followed, err := RecordSubscription(ctx, org, compIn("pc-remind/followed", "go", end.AddDate(0, -1, 0), end))
	if err != nil {
		t.Fatalf("comp: %v", err)
	}
	next := subscription.New(db)
	next.UserId, next.PlanId, next.DefaultPaymentMethod = "pc-remind/followed", "go", "pm_next"
	schedule(next, end)
	if err := next.Create(); err != nil {
		t.Fatalf("scheduled: %v", err)
	}

	var sent []Reminder
	send := func(_ context.Context, r Reminder) error { sent = append(sent, r); return nil }
	pass := func(at time.Time) *RemindRun {
		run := &RemindRun{}
		remindOrg(ctx, org, at, send, run)
		return run
	}
	if run := pass(now); run.Sent != 2 || len(sent) != 2 {
		t.Fatalf("first pass %+v sent %+v, want the wire and the legacy plan reminded", run, sent)
	}
	kinds := map[string]string{}
	for _, r := range sent {
		if r.Lead != 7*24*time.Hour || !r.Ends.Equal(end) {
			t.Fatalf("reminder %+v, want the week's, for %s", r, end)
		}
		kinds[r.SubscriptionID] = r.Kind
	}
	if kinds[wire.Subscription.ID] != RemindExternal || kinds[legacy.Id()] != RemindPrepaid {
		t.Fatalf("kinds = %v, want external and prepaid", kinds)
	}
	if run := pass(now.Add(time.Hour)); run.Sent != 0 {
		t.Fatalf("second pass sent %d, want none: each reminder goes once", run.Sent)
	}

	// A day out, the day's reminder; a send that fails is tried again next pass.
	sent = nil
	fail := true
	send = func(_ context.Context, r Reminder) error {
		if fail && r.SubscriptionID == wire.Subscription.ID {
			return errors.New("notify is down")
		}
		sent = append(sent, r)
		return nil
	}
	day := end.Add(-20 * time.Hour)
	if run := pass(day); run.Sent != 1 || run.Failed != 1 {
		t.Fatalf("day pass %+v, want one sent and one failed", run)
	}
	fail = false
	if run := pass(day.Add(time.Minute)); run.Sent != 1 || sent[len(sent)-1].SubscriptionID != wire.Subscription.ID || sent[len(sent)-1].Lead != 24*time.Hour {
		t.Fatalf("retry pass %+v sent %+v, want the failed day reminder sent", run, sent)
	}
	if run := pass(day.Add(2 * time.Minute)); run.Sent != 0 {
		t.Fatalf("a third pass sent %d", run.Sent)
	}
	for _, r := range sent {
		if r.SubscriptionID == card.Id() || r.SubscriptionID == followed.Subscription.ID {
			t.Fatalf("reminded %s, which renews by card", r.SubscriptionID)
		}
	}
}

// A customer's own price is what the card is charged, at the sale and at every
// renewal, and the invoice shows the plan at list with the difference as their
// discount. A private plan is sold by card only to a customer with a price of it:
// one only the plan authority holds, as staff write it.
// Mutation: drop the agreed step in buildPeriodInvoice and the renewal charges list.
func TestCustomerPrice_TheCardIsChargedTheAgreedPriceEveryPeriod(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	authorityPlan(t, ctx, "patron", "personal", plan.StatusPrivate, 99500)
	org := moneyOrg("pc-price")
	db := datastore.New(org.Namespaced(ctx))
	m := squareMock("cust_up", "ccof_up", "sqpay_up")
	withFakeSquare(t, m)

	if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"patron"}`, nil); r.StatusCode != 404 || m.chargeCalls != 0 {
		t.Fatalf("a private plan with no price agreed: %d %s, charged %d; want 404, nothing charged", r.StatusCode, bodyOf(r), m.chargeCalls)
	}
	if _, err := SetCustomerPrice(ctx, org, PriceIn{Subject: org.Name, Plan: "patron", Cents: 99500}); !IsSaleRefused(err) {
		t.Fatalf("a price at list: %v, want refused", err)
	}
	set, err := SetCustomerPrice(ctx, org, PriceIn{Subject: org.Name, Plan: "patron", Cents: 19500, Note: "agreed with the owner"})
	if err != nil || set.Price.Cents != 19500 || set.Price.ListCents != 99500 {
		t.Fatalf("set: %+v, %v", set, err)
	}
	if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"patron"}`, nil); r.StatusCode != 201 {
		t.Fatalf("sale: %d %s", r.StatusCode, bodyOf(r))
	}
	if m.chargeCalls != 1 || m.lastChargeAmount != 19500 {
		t.Fatalf("sale charged %d time(s), last %d; want 19500", m.chargeCalls, m.lastChargeAmount)
	}
	sub := parentSub(t, db, org.Name, "patron")
	if sub == nil || sub.CustomerCents != 19500 {
		t.Fatalf("row %+v, want it to carry the customer's price", sub)
	}
	check := func(inv *billinginvoice.BillingInvoice) {
		t.Helper()
		if planFeeOf(inv) != 99500 || inv.Discount != 80000 || inv.AmountDue != 19500 || inv.DiscountName != "Customer price: $195.00/month" {
			t.Fatalf("invoice fee %d discount %d %q due %d; want 99500 less 80000 as the customer's price, 19500 due",
				planFeeOf(inv), inv.Discount, inv.DiscountName, inv.AmountDue)
		}
	}
	invs := invoicesForSub(t, db, sub.Id())
	if len(invs) != 1 {
		t.Fatalf("invoices = %d", len(invs))
	}
	check(invs[0])

	due(t, db, sub.Id())
	if out := renewDue(ctx, org, ""); len(out) != 1 || !out[0].Success || out[0].AmountCents != 19500 {
		t.Fatalf("renewal = %+v, want 19500 on the card", out)
	}
	if m.chargeCalls != 2 || m.lastChargeAmount != 19500 {
		t.Fatalf("renewal charged %d time(s), last %d; want 19500", m.chargeCalls, m.lastChargeAmount)
	}
	for _, inv := range invoicesForSub(t, db, sub.Id()) {
		check(inv)
	}
	if prices, err := CustomerPrices(ctx, org); err != nil || len(prices) != 1 || prices[0].Cents != 19500 {
		t.Fatalf("prices = %+v, %v", prices, err)
	}

	// Removed, the next period bills the list price.
	cleared, err := SetCustomerPrice(ctx, org, PriceIn{Subject: org.Name, Plan: "patron"})
	if err != nil || len(cleared.Subscriptions) != 1 || cleared.Subscriptions[0].CustomerCents != 0 {
		t.Fatalf("clear: %+v, %v", cleared, err)
	}
}

// planFeeOf is what an invoice's plan lines bill before any discount.
func planFeeOf(inv *billinginvoice.BillingInvoice) int64 {
	n := int64(0)
	for _, li := range inv.LineItems {
		if li.Type == billinginvoice.LineSubscription {
			n += li.Amount
		}
	}
	return n
}

// Staff move a period's end into the future; a card plan waiting at the old end
// moves with it; an end in the past and a canceled row are refused.
func TestAdjustPeriod_MovesTheEndAndThePlanThatFollows(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := balanceSetup(t, ctx, "pc-adjust", 0)
	start := time.Now().UTC().Truncate(time.Second).AddDate(0, 0, -10)
	end := start.AddDate(0, 0, 20)
	got, err := RecordSubscription(ctx, org, recordIn(balSubject, balSlug, balPrice, start, end))
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	p, err := resolveSubscriptionPlan(db, balSlug)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	next := subscription.New(db)
	next.UserId, next.PlanId, next.Plan, next.DefaultPaymentMethod = balSubject, balSlug, *p, "pm_next"
	schedule(next, end)
	if err := next.Create(); err != nil {
		t.Fatalf("scheduled: %v", err)
	}
	later := end.AddDate(0, 0, 14)
	adj, err := AdjustPeriod(ctx, org, AdjustIn{ID: got.Subscription.ID, End: later, Note: "two weeks for the outage"})
	if err != nil {
		t.Fatalf("adjust: %v", err)
	}
	if !adj.From.Equal(end) || adj.Moved == nil || adj.Moved.ID != next.Id() {
		t.Fatalf("adjust: %+v, %v", adj, err)
	}
	if s := reload(t, db, got.Subscription.ID); !s.PeriodEnd.Equal(later) || len(s.Metadata["adjustments"].([]interface{})) != 1 {
		t.Fatalf("row ends %s with %v, want %s and the move kept", s.PeriodEnd, s.Metadata["adjustments"], later)
	}
	if n := reload(t, db, next.Id()); !n.PeriodStart.Equal(later) || n.Status != subscription.Scheduled {
		t.Fatalf("the plan that follows starts %s (%s), want %s", n.PeriodStart, n.Status, later)
	}
	if _, err := AdjustPeriod(ctx, org, AdjustIn{ID: got.Subscription.ID, End: time.Now().Add(-time.Hour)}); !IsSaleRefused(err) {
		t.Fatalf("an end in the past: %v, want refused", err)
	}
	if _, err := CancelSubscription(ctx, org, AnyHolder, got.Subscription.ID, false); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := AdjustPeriod(ctx, org, AdjustIn{ID: got.Subscription.ID, End: later.AddDate(0, 0, 1)}); !IsSaleRefused(err) {
		t.Fatalf("a canceled row: %v, want refused", err)
	}
	if _, err := AdjustPeriod(ctx, org, AdjustIn{ID: "nope", End: later}); !IsSaleNotFound(err) {
		t.Fatalf("an unknown row: %v, want not found", err)
	}
}

// countingPrepaid is a Prepaid that holds nothing and counts every time the
// engine asks it for money.
type countingPrepaid struct{ calls int }

func (c *countingPrepaid) Available(context.Context, string, currency.Type) (int64, error) {
	c.calls++
	return 0, nil
}

func (c *countingPrepaid) Draw(context.Context, string, currency.Type, int64, string) (engine.Drawn, error) {
	c.calls++
	return engine.Drawn{}, creditledger.ErrShort
}

// A comp of a plan only the plan authority knows confers the tier that plan's row
// gives — paid, Pro; enterprise, Enterprise — and the tier names the plan. A comp
// of a free row, and a self-created internal $0 row, confer nothing. Mutation:
// drop the comp case in paidRow and the patron comp reads Free.
//
// The catalog's own private plan, Unlimited, is the comp staff give at
// admin.hanzo.ai: it confers Pro from the catalog, and names itself.
func TestTier_ACompOfAPrivatePlanConfersItsTier(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	authorityPlan(t, ctx, "patron", "personal", plan.StatusPrivate, 99500)
	authorityPlan(t, ctx, "partner", "enterprise", plan.StatusPrivate, 500000)
	authorityPlan(t, ctx, "sampler", "personal", plan.StatusPrivate, 0)
	start := time.Now().UTC().Truncate(24 * time.Hour)

	unlimited := moneyOrg("tier-comp-unlimited")
	if _, err := RecordSubscription(ctx, unlimited, compIn(unlimited.Name, "unlimited", start, start.AddDate(0, 1, 0))); err != nil {
		t.Fatalf("comp unlimited: %v", err)
	}
	if v, err := ReadTier(ctx, unlimited, unlimited.Name); err != nil || v.Tier.Name != tier.Pro || v.Plan != "unlimited" || v.PlanName != "Unlimited" {
		t.Fatalf("a comp of the catalog's Unlimited: %+v, %v; want Pro, plan unlimited named Unlimited", v, err)
	}

	for slug, want := range map[string]tier.Name{"patron": tier.Pro, "partner": tier.Enterprise, "sampler": tier.Free} {
		org := moneyOrg("tier-comp-" + slug)
		if _, err := RecordSubscription(ctx, org, compIn(org.Name, slug, start, start.AddDate(0, 1, 0))); err != nil {
			t.Fatalf("comp %s: %v", slug, err)
		}
		v, err := ReadTier(ctx, org, org.Name)
		if err != nil {
			t.Fatalf("tier %s: %v", slug, err)
		}
		if v.Tier.Name != want {
			t.Errorf("a comp of %s confers %s, want %s", slug, v.Tier.Name, want)
		}
		if want != tier.Free && (v.Plan != slug || v.PlanName != slug) {
			t.Errorf("a comp of %s answers plan %q named %q", slug, v.Plan, v.PlanName)
		}
	}

	// A row nobody paid for and staff never recorded confers nothing.
	org := moneyOrg("tier-internal")
	db := datastore.New(org.Namespaced(ctx))
	p, err := resolveSubscriptionPlan(db, "patron")
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	free := *p
	free.Price = 0
	s := subscription.New(db)
	s.UserId, s.ProviderType, s.Quantity = org.Name, "internal", 1
	engine.StartSubscription(s, &free)
	if err := s.Create(); err != nil {
		t.Fatalf("create: %v", err)
	}
	if v, err := ReadTier(ctx, org, org.Name); err != nil || v.Tier.Name != tier.Free {
		t.Fatalf("an internal $0 row: tier %+v, %v; want free", v, err)
	}
}

// A payment recorded for a customer with their own price is checked against it,
// and the row carries it.
func TestRecordSubscription_ChecksTheCustomersOwnPrice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := moneyOrg("rec-price")
	start := time.Now().UTC().Truncate(24 * time.Hour)
	in := recordIn(org.Name, "unlimited", 19500, start, start.AddDate(0, 1, 0))
	if _, err := RecordSubscription(ctx, org, in); !IsSaleRefused(err) {
		t.Fatalf("195 against a 995 plan with no price agreed: %v, want refused", err)
	}
	if _, err := SetCustomerPrice(ctx, org, PriceIn{Subject: org.Name, Plan: "unlimited", Cents: 19500}); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := RecordSubscription(ctx, org, in)
	if err != nil || got.Subscription.CustomerCents != 19500 {
		t.Fatalf("record at the customer's price: %+v, %v", got, err)
	}
	next := in
	next.PeriodStart, next.PeriodEnd = in.PeriodEnd, in.PeriodEnd.AddDate(0, 1, 0)
	if got, err := RecordSubscription(ctx, org, next); err != nil || got.Outcome != RecordExtended {
		t.Fatalf("the next period at the customer's price: %+v, %v", got, err)
	}
}
