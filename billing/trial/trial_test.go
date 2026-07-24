package trial

import (
	"context"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transaction"
	txutil "github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

const (
	testPrice  = int64(2000) // $20.00/mo
	testCredit = int64(2000) // $20.00 unified trial credit
)

// useTestPlan wires the $20 entry plan without touching the real catalog, so
// these unit tests never depend on @hanzo/plans.
func useTestPlan() {
	SetEntryPlanResolver(func() Plan {
		return Plan{
			Slug:        PlanSlug,
			Name:        "Starter",
			Description: "Entry plan",
			PriceCents:  testPrice,
			CreditCents: testCredit,
			Currency:    "usd",
		}
	})
}

func newDB(t *testing.T) (context.Context, *datastore.Datastore, func()) {
	t.Helper()
	useTestPlan()
	ctx := ae.NewContext()
	return ctx, datastore.New(ctx), func() { ctx.Close() }
}

func balanceCents(t *testing.T, ctx context.Context, subject string, isTest bool) int64 {
	t.Helper()
	datas, err := txutil.GetTransactionsByCurrency(ctx, subject, Kind, currency.USD, isTest)
	if err != nil {
		t.Fatalf("balance query: %v", err)
	}
	if d, ok := datas.Data[currency.USD]; ok {
		return int64(d.Balance)
	}
	return 0
}

func trialSub(t *testing.T, db *datastore.Datastore, subject string) (*subscription.Subscription, bool) {
	t.Helper()
	sub := subscription.New(db)
	found, err := sub.Query().Filter("UserId=", subject).Get()
	if err != nil {
		t.Fatalf("subscription query: %v", err)
	}
	return sub, found
}

// recordUsage debits the subject's balance the way the cloud gateway does —
// one Withdraw against SourceId=subject regardless of what was consumed.
func recordUsage(t *testing.T, db *datastore.Datastore, subject, tag string, cents int64) {
	t.Helper()
	tr := transaction.New(db)
	tr.Type = transaction.Withdraw
	tr.SourceId = subject
	tr.SourceKind = Kind
	tr.Currency = currency.USD
	tr.Amount = currency.Cents(cents)
	tr.Tags = tag
	if err := tr.Create(); err != nil {
		t.Fatalf("record usage: %v", err)
	}
}

func manualDeposit(t *testing.T, db *datastore.Datastore, subject string, cents int64, tag string) {
	t.Helper()
	tr := transaction.New(db)
	tr.Type = transaction.Deposit
	tr.DestinationId = subject
	tr.DestinationKind = Kind
	tr.Currency = currency.USD
	tr.Amount = currency.Cents(cents)
	tr.Tags = tag
	if err := tr.Create(); err != nil {
		t.Fatalf("manual deposit: %v", err)
	}
}

func assertDaysAway(t *testing.T, from, at time.Time, days int) {
	t.Helper()
	want := time.Duration(days) * 24 * time.Hour
	diff := at.Sub(from)
	if diff < want-2*time.Hour || diff > want+2*time.Hour {
		t.Fatalf("trial end %v is %v from start, want ~%d days", at, diff, days)
	}
}

// (b) New signup with NO card -> 7-day trial of the $20 plan + unified credit.
func TestStart_NoCard_SevenDayTrial(t *testing.T) {
	ctx, db, done := newDB(t)
	defer done()

	const subject = "hanzo/nocard"
	start := time.Now()

	res, err := Start(db, subject, false, false)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !res.Started {
		t.Fatalf("expected Started=true, got %+v", res)
	}
	if res.TrialDays != NoCardTrialDays {
		t.Errorf("TrialDays = %d, want %d", res.TrialDays, NoCardTrialDays)
	}
	if res.CreditCents != testCredit {
		t.Errorf("CreditCents = %d, want %d", res.CreditCents, testCredit)
	}
	assertDaysAway(t, start, res.TrialEnd, NoCardTrialDays)

	sub, found := trialSub(t, db, subject)
	if !found {
		t.Fatal("trial subscription not persisted")
	}
	if sub.Status != subscription.Trialing {
		t.Errorf("Status = %q, want trialing", sub.Status)
	}
	if sub.Plan.Slug != PlanSlug {
		t.Errorf("Plan.Slug = %q, want %q", sub.Plan.Slug, PlanSlug)
	}
	if sub.ProviderType != ProviderType {
		t.Errorf("ProviderType = %q, want %q", sub.ProviderType, ProviderType)
	}

	if got := balanceCents(t, ctx, subject, false); got != testCredit {
		t.Errorf("balance = %d, want %d (funded trial credit)", got, testCredit)
	}
}

func TestStartForStore_IsolatesBillingUnit(t *testing.T) {
	ctx, db, done := newDB(t)
	defer done()

	const subject = "hanzo/multi"
	first, err := StartForStore(db, subject, "store-a", false, false)
	if err != nil || !first.Started {
		t.Fatalf("start store-a: result=%+v err=%v", first, err)
	}
	second, err := StartForStore(db, subject, "store-b", false, false)
	if err != nil || !second.Started {
		t.Fatalf("start store-b: result=%+v err=%v", second, err)
	}
	replay, err := StartForStore(db, subject, "store-a", false, false)
	if err != nil {
		t.Fatalf("replay store-a: %v", err)
	}
	if replay.Started || replay.Reason != "not_new" {
		t.Fatalf("store-a replay = %+v, want idempotent not_new", replay)
	}

	subs := make([]*subscription.Subscription, 0)
	if _, err := subscription.Query(db).Filter("UserId=", subject).GetAll(&subs); err != nil {
		t.Fatalf("query subscriptions: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("subscriptions = %d, want one per store", len(subs))
	}
	seen := map[string]bool{}
	for _, sub := range subs {
		seen[sub.StoreId] = true
	}
	if !seen["store-a"] || !seen["store-b"] {
		t.Fatalf("store bindings = %v", seen)
	}
	// Per-store trial SUBSCRIPTIONS are isolated (two above), but the trial
	// CREDIT is capped to ONE per org subject: the 2nd store reuses the org's
	// existing $20, it does not mint a fresh grant. Otherwise looping
	// create-store -> trial would farm unlimited free credit.
	if got := balanceCents(t, ctx, subject, false); got != testCredit {
		t.Fatalf("trial balance = %d, want %d (one org-capped credit, not per-store)", got, testCredit)
	}
}

// TestStartForStore_TrialCreditCappedPerOrg proves the money cap directly: a
// second store's trial reuses the org's ONE trial credit and mints no new grant,
// so looping create-store -> trial cannot farm free $20s.
func TestStartForStore_TrialCreditCappedPerOrg(t *testing.T) {
	ctx, db, done := newDB(t)
	defer done()

	const subject = "hanzo/farmer"
	first, err := StartForStore(db, subject, "store-a", false, false)
	if err != nil || !first.Started || first.TransactionID == "" {
		t.Fatalf("start store-a: result=%+v err=%v", first, err)
	}
	second, err := StartForStore(db, subject, "store-b", false, false)
	if err != nil {
		t.Fatalf("start store-b: %v", err)
	}
	// The 2nd store still gets its own subscription, but the credit is the SAME
	// row — no new money minted.
	if second.TransactionID != first.TransactionID {
		t.Fatalf("2nd store minted a NEW credit %q (want reuse of %q)", second.TransactionID, first.TransactionID)
	}
	if got := balanceCents(t, ctx, subject, false); got != testCredit {
		t.Fatalf("org wallet = %d after two stores, want %d (one capped credit)", got, testCredit)
	}
}

// (c) New signup WITH a card -> 30-day trial.
func TestStart_WithCard_ThirtyDayTrial(t *testing.T) {
	ctx, db, done := newDB(t)
	defer done()

	const subject = "hanzo/carded"
	start := time.Now()

	res, err := Start(db, subject, true, false)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !res.Started || res.TrialDays != CardTrialDays {
		t.Fatalf("expected 30-day started trial, got %+v", res)
	}
	assertDaysAway(t, start, res.TrialEnd, CardTrialDays)

	sub, found := trialSub(t, db, subject)
	if !found || sub.Status != subscription.Trialing {
		t.Fatalf("expected trialing subscription, found=%v", found)
	}
	assertDaysAway(t, start, sub.TrialEnd, CardTrialDays)

	if got := balanceCents(t, ctx, subject, false); got != testCredit {
		t.Errorf("balance = %d, want %d", got, testCredit)
	}
}

// (d) Adding a card extends a 7-day trial to 30 days (subscription + credit).
func TestExtendForCard_ExtendsSevenToThirty(t *testing.T) {
	ctx, db, done := newDB(t)
	defer done()

	const subject = "hanzo/upgrader"
	start := time.Now()

	if _, err := Start(db, subject, false, false); err != nil {
		t.Fatalf("Start: %v", err)
	}

	res, err := ExtendForCard(db, subject, false)
	if err != nil {
		t.Fatalf("ExtendForCard: %v", err)
	}
	if !res.Extended {
		t.Fatalf("expected Extended=true, got %+v", res)
	}
	if res.TrialDays != CardTrialDays {
		t.Errorf("TrialDays = %d, want %d", res.TrialDays, CardTrialDays)
	}
	assertDaysAway(t, start, res.TrialEnd, CardTrialDays)

	sub, found := trialSub(t, db, subject)
	if !found {
		t.Fatal("subscription missing after extend")
	}
	assertDaysAway(t, sub.TrialStart, sub.TrialEnd, CardTrialDays)

	// Trial credit expiry stretched to the new end — and NOT doubled: still $20.
	dep, ok, err := findTrialCredit(db, subject)
	if err != nil || !ok {
		t.Fatalf("trial credit missing: ok=%v err=%v", ok, err)
	}
	assertDaysAway(t, sub.TrialStart, dep.ExpiresAt, CardTrialDays)
	if got := balanceCents(t, ctx, subject, false); got != testCredit {
		t.Errorf("balance after extend = %d, want %d (unchanged, not doubled)", got, testCredit)
	}
}

// (d') A card added before any trial exists starts a fresh 30-day trial.
func TestExtendForCard_NoTrialYet_StartsThirtyDay(t *testing.T) {
	_, db, done := newDB(t)
	defer done()

	const subject = "hanzo/cardfirst"
	res, err := ExtendForCard(db, subject, false)
	if err != nil {
		t.Fatalf("ExtendForCard: %v", err)
	}
	if !res.Started || res.TrialDays != CardTrialDays {
		t.Fatalf("expected a started 30-day trial, got %+v", res)
	}
	sub, found := trialSub(t, db, subject)
	if !found || sub.Status != subscription.Trialing {
		t.Fatalf("expected trialing subscription, found=%v", found)
	}
}

// (e) The trial allowance is ONE unified balance spendable across compute OR
// AI: both debit the same credit, and Start funds exactly one deposit.
func TestTrialCredit_OneUnifiedBalance(t *testing.T) {
	ctx, db, done := newDB(t)
	defer done()

	const subject = "hanzo/unified"
	if _, err := Start(db, subject, false, false); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if got := balanceCents(t, ctx, subject, false); got != testCredit {
		t.Fatalf("initial balance = %d, want %d", got, testCredit)
	}

	// AI usage and compute usage draw down the SAME balance.
	recordUsage(t, db, subject, "api-usage", 500)     // $5 AI tokens
	recordUsage(t, db, subject, "compute-usage", 800) // $8 droplet time

	if got := balanceCents(t, ctx, subject, false); got != testCredit-1300 {
		t.Errorf("balance = %d, want %d (one balance across compute+AI)", got, testCredit-1300)
	}

	// Exactly one trial-credit deposit — no separate buckets.
	rootKey := db.NewKey("synckey", "", 1, nil)
	deps := make([]*transaction.Transaction, 0)
	if _, err := transaction.Query(db).Ancestor(rootKey).
		Filter("DestinationId=", subject).Filter("Tags=", CreditTag).GetAll(&deps); err != nil {
		t.Fatalf("deposit query: %v", err)
	}
	if len(deps) != 1 {
		t.Errorf("trial-credit deposits = %d, want exactly 1 (single unified credit)", len(deps))
	}
}

// (e') Start is idempotent: a repeat call funds nothing more.
func TestStart_Idempotent(t *testing.T) {
	ctx, db, done := newDB(t)
	defer done()

	const subject = "hanzo/repeat"
	if _, err := Start(db, subject, false, false); err != nil {
		t.Fatalf("Start #1: %v", err)
	}
	res, err := Start(db, subject, false, false)
	if err != nil {
		t.Fatalf("Start #2: %v", err)
	}
	if res.Started {
		t.Errorf("second Start should be a no-op, got %+v", res)
	}
	if res.Reason != "not_new" {
		t.Errorf("Reason = %q, want not_new", res.Reason)
	}
	if got := balanceCents(t, ctx, subject, false); got != testCredit {
		t.Errorf("balance after repeat = %d, want %d (not doubled)", got, testCredit)
	}
}

// (f) Existing users — and Dave's real $10k ledger — are untouched: Start is a
// no-op and the balance is preserved.
func TestStart_ExistingUserUnaffected(t *testing.T) {
	ctx, db, done := newDB(t)
	defer done()

	// Dave: comped with a real $10,000 balance (a manual deposit, no trial tag).
	const dave = "hanzo/dave"
	manualDeposit(t, db, dave, 1000000, "manual-comp")
	before := balanceCents(t, ctx, dave, false)
	if before != 1000000 {
		t.Fatalf("Dave pre-balance = %d, want 1000000", before)
	}

	res, err := Start(db, dave, false, false)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if res.Started {
		t.Fatalf("Start must not touch Dave, got %+v", res)
	}
	if res.Reason != "not_new" {
		t.Errorf("Reason = %q, want not_new", res.Reason)
	}
	if _, found := trialSub(t, db, dave); found {
		t.Error("no trial subscription should be created for Dave")
	}
	if after := balanceCents(t, ctx, dave, false); after != before {
		t.Errorf("Dave balance changed: %d -> %d", before, after)
	}

	// A legacy user who already claimed the old starter credit is also skipped.
	const legacy = "hanzo/legacy"
	manualDeposit(t, db, legacy, 10000, "starter-credit")
	res, err = Start(db, legacy, false, false)
	if err != nil {
		t.Fatalf("Start(legacy): %v", err)
	}
	if res.Started {
		t.Errorf("legacy user with prior credit must be skipped, got %+v", res)
	}
}

// Guard: with no catalog wired, trial ops are safe no-ops (non-billing binaries).
func TestStart_NotConfigured(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	db := datastore.New(ctx)

	SetEntryPlanResolver(nil)
	defer useTestPlan() // restore for any later test sharing process state

	res, err := Start(db, "hanzo/unwired", false, false)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if res.Started || res.Reason != "trial_not_configured" {
		t.Errorf("expected trial_not_configured no-op, got %+v", res)
	}
}
