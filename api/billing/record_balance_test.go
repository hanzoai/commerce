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

// A plan paid from the subject's prepaid balance: staff record it, the period is
// drawn now, and the engine renews it from the same money until the money runs
// out. These pin the draw, the renewal and every refusal.

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

// balanceIn is the record staff make: the plan, its price, one period starting
// today, processor "balance" and no reference.
func balanceIn() RecordIn {
	start := time.Now().UTC().Truncate(24 * time.Hour)
	return RecordIn{
		Subject:     balSubject,
		PlanID:      balSlug,
		PriceCents:  balPrice,
		PeriodStart: start,
		PeriodEnd:   start.AddDate(0, 1, 0),
		Processor:   "Balance",
		Terms:       "prepaid by wire",
	}
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

func TestRecordFromBalance_DrawsThePeriodAndOpensARegularPaidRow(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := balanceSetup(t, ctx, "bal-open", 20000)

	got, err := RecordSubscription(ctx, org, balanceIn())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if got.Outcome != RecordCreated || got.Subscription.Status != string(subscription.Active) {
		t.Fatalf("recorded %+v, want a created active row", got)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 20000-balPrice {
		t.Fatalf("balance = %d, want exactly %d drawn from 20000", b, balPrice)
	}

	s := reload(t, db, got.Subscription.ID)
	if s.Type == subscription.External {
		t.Fatal("a balance-paid row is external; the engine would never renew it")
	}
	if s.CurrentInvoiceId == "" || !subscriptionPaymentBacked(s) {
		t.Fatal("the row carries no paid invoice; a private plan on it reads free")
	}
	inv := billinginvoice.New(db)
	if err := inv.GetById(s.CurrentInvoiceId); err != nil {
		t.Fatalf("load first invoice: %v", err)
	}
	if inv.Status != billinginvoice.Paid || inv.AmountPaid != balPrice || inv.PaymentMethod != "balance" {
		t.Fatalf("first invoice %s paid %d by %q, want paid %d by balance", inv.Status, inv.AmountPaid, inv.PaymentMethod, balPrice)
	}
	in := balanceIn()
	if !s.PeriodStart.Equal(in.PeriodStart) || !s.PeriodEnd.Equal(in.PeriodEnd) {
		t.Fatalf("row is on %s..%s, want the period it paid for (%s..%s)", s.PeriodStart, s.PeriodEnd, in.PeriodStart, in.PeriodEnd)
	}
	ref, _ := s.Metadata["reference"].(map[string]interface{})
	if ref["invoice"] != inv.Id() || ref["ledger"] == nil || ref["ledger"] == "" || s.Metadata["collection"] != "balance" {
		t.Fatalf("metadata %#v, want the ledger entry and the invoice as the reference", s.Metadata)
	}
	if tr, _ := deriveTier(db, balSubject, org.TestMode()); tr != tier.Pro {
		t.Fatalf("tier = %q, want pro", tr)
	}

	// A lost answer retried replays the record; it draws nothing.
	again, err := RecordSubscription(ctx, org, balanceIn())
	if err != nil || again.Outcome != RecordUnchanged || again.Subscription.ID != got.Subscription.ID {
		t.Fatalf("retry: %+v, %v; want the same row unchanged", again, err)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 20000-balPrice {
		t.Fatalf("a retry moved the balance to %d", b)
	}
}

func TestRecordFromBalance_RenewalDrawsThePeriodFromTheBalance(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := balanceSetup(t, ctx, "bal-renew", 20000)
	got, err := RecordSubscription(ctx, org, balanceIn())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	due(t, db, got.Subscription.ID)
	before := reload(t, db, got.Subscription.ID)

	out := renewDue(ctx, org, "")
	if len(out) != 1 || !out[0].Success {
		t.Fatalf("cycle = %+v, want the one due row renewed", out)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 20000-2*balPrice {
		t.Fatalf("balance = %d, want another %d drawn", b, balPrice)
	}

	s := reload(t, db, got.Subscription.ID)
	if s.Status != subscription.Active || !s.PeriodStart.Equal(before.PeriodEnd) {
		t.Fatalf("row %s on %s, want active and moved on to %s", s.Status, s.PeriodStart, before.PeriodEnd)
	}
	invs := invoicesForSub(t, db, s.Id())
	if len(invs) != 2 {
		t.Fatalf("invoices = %d, want the first period's and the renewal's", len(invs))
	}
	for _, inv := range invs {
		if inv.Status != billinginvoice.Paid || inv.AmountPaid != balPrice {
			t.Fatalf("invoice %s %s paid %d, want every period paid %d", inv.Id(), inv.Status, inv.AmountPaid, balPrice)
		}
	}
	if tr, _ := deriveTier(db, balSubject, org.TestMode()); tr != tier.Pro {
		t.Fatalf("tier after renewal = %q, want pro", tr)
	}
}

func TestRecordFromBalance_ShortBalanceIsRefusedAndWritesNothing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	// 1000 of credit and 1000 of balance: 2000 against a 2500 period.
	org, db := balanceSetup(t, ctx, "bal-short", 1000)
	g := grant(t, db, balSubject, 1000)

	_, err := RecordSubscription(ctx, org, balanceIn())
	if !IsSaleDeclined(err) {
		t.Fatalf("err = %v, want declined for an insufficient balance", err)
	}
	if n := len(subsOf(t, db, balSubject)); n != 0 {
		t.Fatalf("a refused record wrote %d subscription(s)", n)
	}
	invs := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := billinginvoice.Query(db).Filter("UserId=", balSubject).GetAll(&invs); err != nil || len(invs) != 0 {
		t.Fatalf("a refused record wrote %d invoice(s) (err %v)", len(invs), err)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 1000 {
		t.Fatalf("balance = %d, want the 1000 untouched", b)
	}
	left := creditgrant.New(db)
	if err := left.GetById(g.Id()); err != nil || left.RemainingCents != 1000 {
		t.Fatalf("credit grant holds %d (err %v), want the 1000 untouched", left.RemainingCents, err)
	}

	// Nothing was held either: once funded, the same record goes through.
	deposit(t, db, balSubject, 500)
	if _, err := RecordSubscription(ctx, org, balanceIn()); err != nil {
		t.Fatalf("funded retry: %v", err)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 0 {
		t.Fatalf("balance = %d, want 1000 of credit and 1500 of balance drawn to nothing", b)
	}
}

func TestRecordFromBalance_ExhaustedBalanceLeavesTheRowPastDue(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	// Enough for the first period and 500 over: the renewal cannot be covered.
	org, db := balanceSetup(t, ctx, "bal-exhaust", balPrice+500)
	got, err := RecordSubscription(ctx, org, balanceIn())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	due(t, db, got.Subscription.ID)

	out := renewDue(ctx, org, "")
	if len(out) != 1 || out[0].Success {
		t.Fatalf("cycle = %+v, want the renewal refused", out)
	}
	s := reload(t, db, got.Subscription.ID)
	if s.Status != subscription.PastDue {
		t.Fatalf("status = %s, want past_due", s.Status)
	}
	if tr, _ := deriveTier(db, balSubject, org.TestMode()); tr != tier.Free {
		t.Fatalf("tier = %q, want free once the period is unpaid", tr)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 500 {
		t.Fatalf("balance = %d, want the 500 left untouched by a renewal it cannot cover", b)
	}

	// The next sweep neither charges again nor lets the row back up.
	if out := renewDue(ctx, org, ""); len(out) != 1 || out[0].Success {
		t.Fatalf("second sweep = %+v, want still unpaid", out)
	}
	if s := reload(t, db, got.Subscription.ID); s.Status != subscription.PastDue {
		t.Fatalf("status after a second sweep = %s, want past_due", s.Status)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 500 {
		t.Fatalf("balance after a second sweep = %d, want 500", b)
	}
}

func TestRecordFromBalance_Refusals(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := balanceSetup(t, ctx, "bal-refuse", 20000)

	wrong := balanceIn()
	wrong.PriceCents = balPrice - 1
	if _, err := RecordSubscription(ctx, org, wrong); !IsSaleRefused(err) {
		t.Fatalf("a price that is not the plan's: err=%v, want refused", err)
	}
	long := balanceIn()
	long.PeriodEnd = long.PeriodEnd.AddDate(0, 0, 15)
	if _, err := RecordSubscription(ctx, org, long); !IsSaleRefused(err) {
		t.Fatalf("a period longer than the plan's: err=%v, want refused", err)
	}
	short := balanceIn()
	short.PeriodEnd = short.PeriodEnd.AddDate(0, 0, -4)
	if _, err := RecordSubscription(ctx, org, short); !IsSaleRefused(err) {
		t.Fatalf("a period shorter than any month: err=%v, want refused", err)
	}
	past := balanceIn()
	past.PeriodStart, past.PeriodEnd = past.PeriodStart.AddDate(0, -2, 0), past.PeriodStart.AddDate(0, -1, 0)
	if _, err := RecordSubscription(ctx, org, past); !IsSaleRefused(err) {
		t.Fatalf("a period that has ended: err=%v, want refused", err)
	}
	ahead := balanceIn()
	ahead.PeriodStart, ahead.PeriodEnd = ahead.PeriodStart.AddDate(0, 6, 0), ahead.PeriodStart.AddDate(0, 7, 0)
	if _, err := RecordSubscription(ctx, org, ahead); !IsSaleRefused(err) {
		t.Fatalf("a period that has not begun: err=%v, want refused", err)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 20000 {
		t.Fatalf("refusals moved the balance to %d", b)
	}

	// A subject already holding a plan is refused, and nothing is drawn.
	liveSub(t, db, balSubject, "dev", "square")
	if _, err := RecordSubscription(ctx, org, balanceIn()); !IsSaleConflict(err) {
		t.Fatalf("a subject already holding a plan: err=%v, want conflict", err)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 20000 {
		t.Fatalf("a refused record moved the balance to %d", b)
	}
}

// TestRecordFromBalance_AShorterMonthMayEndThePeriodEarly: the calendar ends a
// period starting on the 31st on the last day of a shorter month, up to three days
// before engine.Advance does, and that period is one of the plan's.
func TestRecordFromBalance_AShorterMonthMayEndThePeriodEarly(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, _ := balanceSetup(t, ctx, "bal-monthend", 20000)
	in := balanceIn()
	in.PeriodEnd = in.PeriodEnd.AddDate(0, 0, -3)
	if _, err := RecordSubscription(ctx, org, in); err != nil {
		t.Fatalf("a period three days short of Advance: %v", err)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 20000-balPrice {
		t.Fatalf("balance = %d, want %d drawn", b, balPrice)
	}
}

// TestRenewDue_AMissedRowPaysOnlyThePeriodRunningNow: a row the sweep missed for
// whole periods is billed once, for the period running now. The periods that ended
// in between are not billed after the fact.
func TestRenewDue_AMissedRowPaysOnlyThePeriodRunningNow(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := balanceSetup(t, ctx, "bal-behind", 20000)
	got, err := RecordSubscription(ctx, org, balanceIn())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	s := reload(t, db, got.Subscription.ID)
	s.PeriodStart, s.PeriodEnd = time.Now().AddDate(0, -4, 0), time.Now().AddDate(0, -3, 0)
	if err := s.Update(); err != nil {
		t.Fatalf("save: %v", err)
	}

	out := renewDue(ctx, org, "")
	if len(out) != 1 || !out[0].Success {
		t.Fatalf("sweep = %+v, want the row renewed once", out)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 20000-2*balPrice {
		t.Fatalf("balance = %d, want one more period drawn, not one for every period missed", b)
	}
	r := reload(t, db, got.Subscription.ID)
	if now := time.Now(); r.Status != subscription.Active || r.PeriodStart.After(now) || !r.PeriodEnd.After(now) {
		t.Fatalf("row %s on %s..%s, want active on the period running now", r.Status, r.PeriodStart, r.PeriodEnd)
	}
	if out := renewDue(ctx, org, ""); len(out) != 0 {
		t.Fatalf("a second sweep renewed again: %+v", out)
	}
}

// TestRenewDue_CanceledAtPeriodEndIsNotCharged: a plan canceled at the end of its
// period ends when the period does. The sweep cancels it; it draws nothing.
func TestRenewDue_CanceledAtPeriodEndIsNotCharged(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := balanceSetup(t, ctx, "bal-endcancel", 20000)
	got, err := RecordSubscription(ctx, org, balanceIn())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if _, err := CancelSubscription(ctx, org, balSubject, got.Subscription.ID, true); err != nil {
		t.Fatalf("cancel at period end: %v", err)
	}
	due(t, db, got.Subscription.ID)

	if out := renewDue(ctx, org, ""); len(out) != 1 || out[0].Success || out[0].InvoiceId != "" {
		t.Fatalf("sweep = %+v, want the row ended, not renewed", out)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 20000-balPrice {
		t.Fatalf("balance = %d, want nothing drawn after a cancel", b)
	}
	if s := reload(t, db, got.Subscription.ID); s.Status != subscription.Canceled || s.Ended.IsZero() {
		t.Fatalf("row %s ended %s, want canceled as of its period's end", s.Status, s.Ended)
	}
	if out := renewDue(ctx, org, ""); len(out) != 0 {
		t.Fatalf("a canceled row was swept again: %+v", out)
	}
}

// TestRenewDue_PayingThePastDueInvoiceBringsThePlanBack: the balance runs out, the
// row goes past due, the org is funded and the invoice paid. The next sweep reads
// the paid invoice and puts the plan back on its next period.
func TestRenewDue_PayingThePastDueInvoiceBringsThePlanBack(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := balanceSetup(t, ctx, "bal-revive", balPrice)
	got, err := RecordSubscription(ctx, org, balanceIn())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	due(t, db, got.Subscription.ID)
	out := renewDue(ctx, org, "")
	if len(out) != 1 || out[0].Success || out[0].InvoiceId == "" {
		t.Fatalf("sweep = %+v, want an unpaid invoice", out)
	}
	dry := reload(t, db, got.Subscription.ID)
	if dry.Status != subscription.PastDue {
		t.Fatalf("status = %s, want past_due", dry.Status)
	}

	// Twenty days go by past due, with no plan served, before the customer pays.
	owedInv := billinginvoice.New(db)
	if err := owedInv.GetById(out[0].InvoiceId); err != nil {
		t.Fatalf("load invoice: %v", err)
	}
	owedInv.PeriodStart, owedInv.PeriodEnd = time.Now().AddDate(0, 0, -20), time.Now().AddDate(0, 0, 10)
	if err := owedInv.Update(); err != nil {
		t.Fatalf("save invoice: %v", err)
	}
	dry.PeriodStart, dry.PeriodEnd = dry.PeriodStart.AddDate(0, 0, -20), owedInv.PeriodStart
	if err := dry.Update(); err != nil {
		t.Fatalf("save row: %v", err)
	}

	deposit(t, db, balSubject, balPrice)
	paid, f := CollectInvoice(ctx, org, out[0].InvoiceId, nil, nil)
	if f != nil || !paid.Paid || paid.BalanceUsedCents != balPrice {
		t.Fatalf("pay the invoice: %+v, %v", paid, f)
	}
	if out := renewDue(ctx, org, ""); len(out) != 1 || !out[0].Success {
		t.Fatalf("sweep after paying = %+v, want the paid period read back", out)
	}
	// The payment buys a whole period from when it was made, not the ten days left
	// of the invoice's.
	s := reload(t, db, got.Subscription.ID)
	if now := time.Now(); s.Status != subscription.Active || now.Sub(s.PeriodStart) > time.Minute || s.PeriodEnd.Before(now.AddDate(0, 0, 27)) {
		t.Fatalf("row %s on %s..%s, want active on a whole period from the payment", s.Status, s.PeriodStart, s.PeriodEnd)
	}
	if tr, _ := deriveTier(db, balSubject, org.TestMode()); tr != tier.Pro {
		t.Fatalf("tier = %q, want pro", tr)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 0 {
		t.Fatalf("balance = %d, want every cent paid once", b)
	}

	// The paid invoice stays in the customer's list, and the next one is numbered
	// after it rather than over it.
	listed, err := ListInvoices(ctx, org, balSubject, "", "", 0, 0)
	if err != nil || len(listed) != 2 {
		t.Fatalf("invoices listed = %d (err %v), want the first period's and the one paid by hand", len(listed), err)
	}
	deposit(t, db, balSubject, balPrice)
	next := reload(t, db, got.Subscription.ID)
	next.PeriodStart, next.PeriodEnd = time.Now().AddDate(0, -1, -2), time.Now().Add(-2*time.Hour)
	if err := next.Update(); err != nil {
		t.Fatalf("save: %v", err)
	}
	if out := renewDue(ctx, org, ""); len(out) != 1 || !out[0].Success {
		t.Fatalf("next renewal = %+v", out)
	}
	numbers := map[string]bool{}
	for _, inv := range invoicesForSub(t, db, got.Subscription.ID) {
		if numbers[inv.NumberStr] {
			t.Fatalf("invoice number %s issued twice", inv.NumberStr)
		}
		numbers[inv.NumberStr] = true
	}
	if len(numbers) != 3 {
		t.Fatalf("invoices = %d, want three distinct", len(numbers))
	}
}

// TestRenewDue_ASandboxRowIsNotPaidFromLiveMoney: an org's prepaid money is its
// own mode's, so a sandbox row in a live org is not renewed from it.
func TestRenewDue_ASandboxRowIsNotPaidFromLiveMoney(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := balanceSetup(t, ctx, "bal-sandbox", 20000)
	got, err := RecordSubscription(ctx, org, balanceIn())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	s := reload(t, db, got.Subscription.ID)
	s.Test = true
	s.PeriodStart, s.PeriodEnd = time.Now().AddDate(0, -1, -1), time.Now().Add(-time.Hour)
	if err := s.Update(); err != nil {
		t.Fatalf("save: %v", err)
	}
	if out := renewDue(ctx, org, ""); len(out) != 0 {
		t.Fatalf("sweep = %+v, want the sandbox row left alone", out)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 20000-balPrice {
		t.Fatalf("live balance = %d; a sandbox row drew from it", b)
	}
}

// TestRecordFromBalance_APastDuePlanBlocksASecond: a plan gone past due still owes
// its period, and paying it brings it back; a second plan beside it would renew
// from the same balance twice.
func TestRecordFromBalance_APastDuePlanBlocksASecond(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := balanceSetup(t, ctx, "bal-second", balPrice+500)
	got, err := RecordSubscription(ctx, org, balanceIn())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	due(t, db, got.Subscription.ID)
	renewDue(ctx, org, "")
	deposit(t, db, balSubject, 20000)

	second := balanceIn()
	second.PeriodStart = second.PeriodStart.Add(time.Hour)
	second.PeriodEnd = second.PeriodEnd.Add(time.Hour)
	if _, err := RecordSubscription(ctx, org, second); !IsSaleConflict(err) {
		t.Fatalf("a second plan beside a past-due one: err=%v, want conflict", err)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 20500 {
		t.Fatalf("balance = %d, want nothing drawn by the refused record", b)
	}
}

// TestCollectInvoice_ADrawThatLandedIsNotTakenTwice: the ledger posts a draw and
// the answer is lost. The next attempt on the same invoice asks under the same
// key, finds that posting, and takes nothing more.
func TestCollectInvoice_ADrawThatLandedIsNotTakenTwice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := balanceSetup(t, ctx, "bal-lost", 0)
	led := &lossy{fakeLedger: newFakeLedger()}
	injectLedger(t, led)
	subject := org.Name
	if _, _, err := led.Credit(ctx, creditledger.CreditInput{Org: org.Name, Currency: "usd", AmountCents: 20000, IdempotencyKey: "g"}); err != nil {
		t.Fatalf("grant: %v", err)
	}
	in := balanceIn()
	in.Subject = subject
	got, err := RecordSubscription(ctx, org, in)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	due(t, db, got.Subscription.ID)

	led.lose = true // the renewal's debit lands, and its answer does not
	out := renewDue(ctx, org, "")
	if len(out) != 1 || out[0].Success {
		t.Fatalf("sweep = %+v, want the lost answer read as unpaid", out)
	}
	// Credit arrives before the retry, so a split measured anew would differ from
	// the one already posted. The retry posts the recorded one.
	g := grant(t, db, subject, 1000)
	if paid, f := CollectInvoice(ctx, org, out[0].InvoiceId, nil, nil); f != nil || !paid.Paid {
		t.Fatalf("collect again: %+v, %v", paid, f)
	}
	if bal, _ := led.Balance(ctx, org.Name, subject, "usd", false); bal != 20000-2*balPrice {
		t.Fatalf("ledger = %d, want the record and ONE renewal drawn", bal)
	}
	left := creditgrant.New(db)
	if err := left.GetById(g.Id()); err != nil || left.RemainingCents != 1000 {
		t.Fatalf("credit grant holds %d (err %v); the retry burned credit the posted split never used", left.RemainingCents, err)
	}
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

// lapsed records a plan the balance pays once, lets its renewal fail, and then
// moves the unpaid period two months into the past, as if it had gone by unserved.
// It answers the row and the unpaid invoice.
func lapsed(t *testing.T, ctx context.Context, org *organization.Organization, db *datastore.Datastore) (*subscription.Subscription, *billinginvoice.BillingInvoice) {
	t.Helper()
	got, err := RecordSubscription(ctx, org, balanceIn())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	due(t, db, got.Subscription.ID)
	out := renewDue(ctx, org, "")
	if len(out) != 1 || out[0].Success {
		t.Fatalf("sweep = %+v, want the renewal unpaid", out)
	}
	inv := billinginvoice.New(db)
	if err := inv.GetById(out[0].InvoiceId); err != nil {
		t.Fatalf("load invoice: %v", err)
	}
	back := time.Now().AddDate(0, -2, 0)
	inv.PeriodStart, inv.PeriodEnd = back, back.AddDate(0, 1, 0)
	if err := inv.Update(); err != nil {
		t.Fatalf("save invoice: %v", err)
	}
	s := reload(t, db, got.Subscription.ID)
	s.PeriodStart, s.PeriodEnd = back.AddDate(0, -1, 0), back
	if err := s.Update(); err != nil {
		t.Fatalf("save row: %v", err)
	}
	return s, inv
}

// TestRenewDue_APeriodThatWentByUnpaidIsNotOwed: a past-due row's unpaid period
// passed with no plan served. It is owed no longer — its invoice is void — and the
// sweep bills the period running now.
func TestRenewDue_APeriodThatWentByUnpaidIsNotOwed(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := balanceSetup(t, ctx, "bal-lapse", balPrice)
	s, stale := lapsed(t, ctx, org, db)

	deposit(t, db, balSubject, balPrice)
	if out := renewDue(ctx, org, ""); len(out) != 1 || !out[0].Success || out[0].InvoiceId == stale.Id() {
		t.Fatalf("sweep = %+v, want the running period billed and paid", out)
	}
	old := billinginvoice.New(db)
	if err := old.GetById(stale.Id()); err != nil || old.Status != billinginvoice.Void {
		t.Fatalf("the lapsed period's invoice is %s (err %v), want void", old.Status, err)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 0 {
		t.Fatalf("balance = %d, want one period drawn for the period running now, none for the one gone by", b)
	}
	r := reload(t, db, s.Id())
	if now := time.Now(); r.Status != subscription.Active || r.PeriodStart.After(now) || !r.PeriodEnd.After(now) {
		t.Fatalf("row %s on %s..%s, want active on the period running now", r.Status, r.PeriodStart, r.PeriodEnd)
	}
}

// TestRenewDue_APeriodPaidAfterItEndedBuysTheNextOne: an unpaid period paid by hand
// only after it was over was never served. The payment buys the period that starts
// when it was made, and the sweep takes nothing more for it.
func TestRenewDue_APeriodPaidAfterItEndedBuysTheNextOne(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := balanceSetup(t, ctx, "bal-late", balPrice)
	s, stale := lapsed(t, ctx, org, db)

	deposit(t, db, balSubject, balPrice)
	if paid, f := CollectInvoice(ctx, org, stale.Id(), nil, nil); f != nil || !paid.Paid {
		t.Fatalf("pay the lapsed invoice: %+v, %v", paid, f)
	}
	if out := renewDue(ctx, org, ""); len(out) != 1 || !out[0].Success || out[0].InvoiceId != stale.Id() {
		t.Fatalf("sweep = %+v, want the paid invoice read back", out)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 0 {
		t.Fatalf("balance = %d, want the one payment and nothing more", b)
	}
	r := reload(t, db, s.Id())
	if now := time.Now(); r.Status != subscription.Active || r.PeriodStart.After(now) || !r.PeriodEnd.After(now) {
		t.Fatalf("row %s on %s..%s, want active on the period the late payment bought", r.Status, r.PeriodStart, r.PeriodEnd)
	}
	if out := renewDue(ctx, org, ""); len(out) != 0 {
		t.Fatalf("a second sweep billed again: %+v", out)
	}
}

// TestCollectInvoice_OnePeriodIsDrawnOnce: two invoices raised for one period —
// two processes renewing the same row — draw from prepaid money once between them.
func TestCollectInvoice_OnePeriodIsDrawnOnce(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := balanceSetup(t, ctx, "bal-twice", 20000)
	got, err := RecordSubscription(ctx, org, balanceIn())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	due(t, db, got.Subscription.ID)
	out := renewDue(ctx, org, "")
	if len(out) != 1 || !out[0].Success {
		t.Fatalf("sweep = %+v", out)
	}
	first := billinginvoice.New(db)
	if err := first.GetById(out[0].InvoiceId); err != nil {
		t.Fatalf("load: %v", err)
	}
	second := billinginvoice.New(db)
	second.UserId, second.SubscriptionId, second.Currency = first.UserId, first.SubscriptionId, first.Currency
	second.PeriodStart, second.PeriodEnd = first.PeriodStart, first.PeriodEnd
	second.LineItems = first.LineItems
	second.RecalculateSubtotal()
	if err := second.Finalize(); err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if err := second.Create(); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, f := CollectInvoice(ctx, org, second.Id(), nil, nil); f != nil {
		t.Fatalf("collect the second invoice: %v", f)
	}
	if b := walletOf(t, ctx, org, balSubject); b != 20000-2*balPrice {
		t.Fatalf("balance = %d, want the record and ONE draw for the period", b)
	}
}

// TestRecordSubscription_ExternalPathDrawsNothing: a wire or a processor's
// invoice still opens an external row that never draws, even for a subject
// whose balance could pay. It is the path this change must leave alone.
func TestRecordSubscription_ExternalPathDrawsNothing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := balanceSetup(t, ctx, "bal-wire", 20000)
	in := balanceIn()
	in.Processor = "wire"
	if _, err := RecordSubscription(ctx, org, in); !IsSaleRefused(err) {
		t.Fatalf("a wire with no reference: err=%v, want refused", err)
	}
	in.Reference = map[string]string{"payment": "FED-1"}
	got, err := RecordSubscription(ctx, org, in)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	s := reload(t, db, got.Subscription.ID)
	if s.Type != subscription.External || s.CurrentInvoiceId != "" || s.Metadata["collection"] != "external" {
		t.Fatalf("type=%q invoice=%q collection=%v, want an external row with no invoice", s.Type, s.CurrentInvoiceId, s.Metadata["collection"])
	}
	if b := walletOf(t, ctx, org, balSubject); b != 20000 {
		t.Fatalf("a wire record moved the balance to %d", b)
	}
	if engine.IsDue(s, s.PeriodEnd.Add(time.Hour)) {
		t.Fatal("an external row answered due")
	}
}

// TestRecordFromBalance_TheGrantedLedgerPaysAndTheTierSeesIt: embedded, the
// balance is the host's ledger — where a staff prepaid grant lands. That grant
// pays the record and the renewal, and the tier reads what is left from the
// same account.
func TestRecordFromBalance_TheGrantedLedgerPaysAndTheTierSeesIt(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := balanceSetup(t, ctx, "bal-ledger", 0)
	led := newFakeLedger()
	injectLedger(t, led)
	subject := org.Name // the org's own account: the pool a grant with no user lands in

	// The staff grant: 20000 of prepaid credit on the org's pool account.
	if _, _, err := led.Credit(ctx, creditledger.CreditInput{
		Org: org.Name, Currency: "usd", Reason: "wire", Tag: "admin-grant", AmountCents: 20000, IdempotencyKey: "grant-1",
	}); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if v, err := ReadTier(ctx, org, subject, tier.Free); err != nil || v.Balance.PrepaidAvailable != 20000 {
		t.Fatalf("tier before: %+v, %v; the tier must read the granted 20000", v.Balance, err)
	}

	in := balanceIn()
	in.Subject = subject
	got, err := RecordSubscription(ctx, org, in)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if bal, _ := led.Balance(ctx, org.Name, subject, "usd", false); bal != 20000-balPrice {
		t.Fatalf("ledger = %d, want %d drawn from the grant", bal, balPrice)
	}
	if n := len(led.debits); n != 1 || led.debits[0].AmountCents != balPrice || led.debits[0].Org != org.Name || led.debits[0].Subject != subject {
		t.Fatalf("ledger debits %+v, want one of %d at %s/%s", led.debits, balPrice, org.Name, subject)
	}
	v, err := ReadTier(ctx, org, subject, tier.Free)
	if err != nil || v.Balance.PrepaidAvailable != 20000-balPrice || v.Balance.EffectiveAvailable != 20000-balPrice {
		t.Fatalf("tier after: %+v, %v; want %d prepaid", v.Balance, err, 20000-balPrice)
	}
	if tr, _ := TierOf(ctx, org, subject); tr != tier.Pro {
		t.Fatalf("tier = %q, want pro", tr)
	}

	// Commerce's own transactions are not the balance here: nothing was written there.
	txs := make([]*transaction.Transaction, 0)
	if _, err := transaction.Query(db).Filter("SourceId=", subject).GetAll(&txs); err != nil || len(txs) != 0 {
		t.Fatalf("commerce's own ledger holds %d withdrawal(s) (err %v); the draw belongs on the host's", len(txs), err)
	}

	due(t, db, got.Subscription.ID)
	if out := renewDue(ctx, org, ""); len(out) != 1 || !out[0].Success {
		t.Fatalf("renewal = %+v, want paid from the ledger", out)
	}
	if bal, _ := led.Balance(ctx, org.Name, subject, "usd", false); bal != 20000-2*balPrice {
		t.Fatalf("ledger after renewal = %d, want another %d drawn", bal, balPrice)
	}
}

// TestSubscribeFromBalance_ShortTouchesNothing: the self-serve purchase draws
// through the same all-or-nothing draw. Credits that cannot finish the job are
// not burned for a sale that is refused.
func TestSubscribeFromBalance_ShortTouchesNothing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := moneyOrg("bal-self")
	db := datastore.New(org.Namespaced(ctx))
	p, err := resolveSubscriptionPlan(db, "dev")
	if err != nil || p.Price <= 1 {
		t.Fatalf("the catalog must sell a paid dev plan: %v", err)
	}
	g := grant(t, db, org.Name, int64(p.Price)-1)

	_, err = Subscribe(ctx, org, SubscribeIn{SourceID: "balance", PlanID: "dev", Subject: org.Name})
	if !IsSaleDeclined(err) {
		t.Fatalf("err = %v, want declined", err)
	}
	left := creditgrant.New(db)
	if err := left.GetById(g.Id()); err != nil || left.RemainingCents != int64(p.Price)-1 {
		t.Fatalf("credit grant holds %d (err %v), want all %d left after a refused sale", left.RemainingCents, err, int64(p.Price)-1)
	}
	if n := len(subsOf(t, db, org.Name)); n != 0 {
		t.Fatalf("a refused sale wrote %d subscription(s)", n)
	}

	deposit(t, db, org.Name, 1)
	sale, err := Subscribe(ctx, org, SubscribeIn{SourceID: "balance", PlanID: "dev", Subject: org.Name})
	if err != nil || sale.AmountCents != int64(p.Price) {
		t.Fatalf("funded sale: %+v, %v", sale, err)
	}
	if b := walletOf(t, ctx, org, org.Name); b != 0 {
		t.Fatalf("balance after the sale = %d, want the 1 cent the credits did not cover drawn", b)
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
