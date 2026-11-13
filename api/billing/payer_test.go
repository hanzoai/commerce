package billing

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/autorecharge"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// ─── the rule belongs to its payer ──────────────────────────────────────────

// ultraReload is the reload the tests' reload plans state: $50 below $10.
var ultraReload = &plan.Reload{ThresholdCents: 1000, AmountCents: 5000}

// payerOrg persists an organization the sweep finds.
func payerOrg(t *testing.T, ctx ae.Context, name string) *organization.Organization {
	t.Helper()
	org := organization.New(datastore.New(ctx))
	org.Name = name
	org.Live = true
	if err := org.Create(); err != nil {
		t.Fatalf("create org: %v", err)
	}
	return org
}

// payerRule gives payer in org a card of its own and a rule switched on.
func payerRule(t *testing.T, ctx ae.Context, org *organization.Organization, payer string, amount, threshold int64) {
	t.Helper()
	db := datastore.New(org.Namespaced(ctx))
	seedSavedCard(t, db, payer, "ccof_"+payer, "cust_"+payer)
	cfg := autorecharge.New(db)
	cfg.UserId, cfg.Enabled, cfg.AmountCents, cfg.ThresholdCents, cfg.Currency = payer, true, amount, threshold, "usd"
	if err := cfg.Create(); err != nil {
		t.Fatalf("create rule for %s: %v", payer, err)
	}
}

// A person who pays from a wallet of their own has a rule of their own: the sweep
// reads that wallet's balance, charges that person's card and credits that
// wallet, beside the org's own account and its own card.
func TestEachPayerReloadsFromItsOwnCard(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := payerOrg(t, ctx, "hanzo")
	payerRule(t, ctx, org, "hanzo/alice", 700, 1000000)
	payerRule(t, ctx, org, "hanzo", 900, 1000000)
	m := squareMock("", "", "sqpay_payers")
	withFakeSquare(t, m)

	run := rechargeRun(t, ctx)
	if run.Charged != 2 || m.chargeCalls != 2 {
		t.Fatalf("one sweep charged %d payers (%d card calls), want both", run.Charged, m.chargeCalls)
	}
	byPayer := map[string]RechargeResult{}
	for _, r := range run.Results {
		byPayer[r.UserId] = r
	}
	if r := byPayer["hanzo/alice"]; !r.Charged || r.AmountCents != 700 {
		t.Fatalf("alice's reload is %+v, want 700 charged", r)
	}
	if got := balanceOf(t, ctx, org, "hanzo/alice"); got != 700 {
		t.Fatalf("alice's wallet holds %d, want her 700", got)
	}
	if got := balanceOf(t, ctx, org, "hanzo"); got != 900 {
		t.Fatalf("the org's account holds %d, want its own 900", got)
	}
	tokens := map[string]bool{}
	for key := range m.chargedKeys {
		tokens[key] = true
	}
	if len(tokens) != 2 {
		t.Fatalf("the two payers charged under %d keys, want 2", len(tokens))
	}
}

// The rule a peer reads and writes is the payer it names: "" is the org's own
// account, "<org>/<name>" a person's wallet, and an account outside the org is
// refused.
func TestARuleIsReadAndWrittenForItsPayer(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("hanzo")
	seedSavedCard(t, datastore.New(org.Namespaced(ctx)), "hanzo/bob", "ccof_bob", "cust_bob")
	edit := AutoRechargeEdit{Enabled: true, ThresholdCents: 1000, AmountCents: 2500}

	if _, err := WriteAutoRecharge(ctx, org, "", edit); !errors.Is(err, ErrNoDefaultPaymentMethod) {
		t.Fatalf("the org's own account, with no card, armed: %v", err)
	}
	v, err := WriteAutoRecharge(ctx, org, "hanzo/bob", edit)
	if err != nil || !v.Enabled || v.Subject != "hanzo/bob" {
		t.Fatalf("bob's rule = %+v, %v; want his, on", v, err)
	}
	if v, err := ReadAutoRecharge(ctx, org, ""); err != nil || v.Enabled || v.Subject != "hanzo" {
		t.Fatalf("the org's own rule reads %+v, %v; want the org's, off", v, err)
	}
	if v, err := ReadAutoRecharge(ctx, org, "Hanzo/Bob"); err != nil || !v.Enabled {
		t.Fatalf("bob's rule reads %+v, %v; want on", v, err)
	}
	for _, foreign := range []string{"acme", "acme/bob", "hanzo/"} {
		if _, err := WriteAutoRecharge(ctx, org, foreign, edit); err == nil {
			t.Errorf("a rule was written for %q, outside hanzo", foreign)
		}
	}
}

// ─── the duplicate guard ───────────────────────────────────────────────────

// Two legitimate reloads in a row: a reload, the balance drained again, a sweep —
// each charges, at the same amount and inside the same fifteen minutes, under a
// key of its own.
func TestADrainAfterAReloadIsANewTrigger(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := seedRechargeOrg(t, ctx, "trigger-again", 500, 1000000)
	m := squareMock("", "", "sqpay_again")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))

	rechargeRun(t, ctx)
	spend(db, org.Name, 100, "usage")
	rechargeRun(t, ctx)
	if m.chargeCalls != 2 || len(m.chargedKeys) != 2 {
		t.Fatalf("two triggers charged %d times under %d keys, want 2 and 2", m.chargeCalls, len(m.chargedKeys))
	}
	if got := storedRule(t, ctx, org); got.Seq != 2 {
		t.Fatalf("the rule closed %d triggers, want 2", got.Seq)
	}
}

// One trigger is charged once: a sweep that finds the balance where the reload
// left it charges nothing, and a sweep retried after the reload's record was lost
// asks under the same key, so the charge guard answers with the first charge.
func TestOneTriggerIsChargedOnce(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := seedRechargeOrg(t, ctx, "trigger-once", 500, 1000000)
	m := squareMock("", "", "sqpay_once")
	withFakeSquare(t, m)

	rechargeRun(t, ctx)
	if run := rechargeRun(t, ctx); run.Charged != 0 || m.chargeCalls != 1 {
		t.Fatalf("the reloaded trigger was swept again: charged %d, card calls %d", run.Charged, m.chargeCalls)
	}

	// The record of the reload is lost: the rule still names the first trigger.
	cfg := storedRule(t, ctx, org)
	cfg.Seq, cfg.After, cfg.Txn = 0, 0, ""
	if err := cfg.Update(); err != nil {
		t.Fatal(err)
	}
	rechargeRun(t, ctx)
	if m.chargeCalls != 1 {
		t.Fatalf("a retried trigger charged the card %d times, want once", m.chargeCalls)
	}
	if got := balanceOf(t, ctx, org, org.Name); got != 500 {
		t.Fatalf("the balance holds %d, want one 500 reload", got)
	}
	if got := storedRule(t, ctx, org); got.Seq != 1 {
		t.Fatalf("the retried trigger was not closed: seq %d", got.Seq)
	}
}

// A card the processor declines is asked again under a new key, and at most
// ReloadsPerDay times a day, on a plan that bounds nothing as on any other.
func TestADecliningCardIsBoundedOnAnUnboundedPlan(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	zero := 0
	reloadPlan(t, ctx, "ultra", 19500, &zero, nil)
	onPlan(t, ctx, "trigger-decline", "ultra")
	m := squareMock("", "", "sqpay_decline_u")
	m.chargeErr = &processor.Answer{Err: errors.New("card declined")}
	withFakeSquare(t, m)

	for i := 0; i < ReloadsPerDay+2; i++ {
		rechargeRun(t, ctx)
	}
	if m.chargeCalls != ReloadsPerDay || len(m.chargedKeys) != ReloadsPerDay {
		t.Fatalf("a declining card was asked %d times under %d keys, want %d and %d",
			m.chargeCalls, len(m.chargedKeys), ReloadsPerDay, ReloadsPerDay)
	}
	run := rechargeRun(t, ctx)
	if len(run.Results) != 1 || !strings.Contains(run.Results[0].Error, "declined 3 times today") {
		t.Fatalf("the skipped reload said %+v, want the declines named", run.Results)
	}
}

// With a host ledger the balance the sweep reads is the host's, and a reload is
// credited there — at the payer's address, as cash, under the processor's
// reference — before it is recorded; a retry of a lost record credits nothing
// twice.
func TestAReloadFundsTheLedgerTheGateReads(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	led := newFakeLedger()
	injectLedger(t, led)
	org := payerOrg(t, ctx, "hanzo")
	payerRule(t, ctx, org, "hanzo/carol", 2500, 1000)
	m := squareMock("", "", "sqpay_host")
	withFakeSquare(t, m)

	run := rechargeRun(t, ctx)
	if run.Charged != 1 || m.chargeCalls != 1 {
		t.Fatalf("an empty host wallet was not reloaded: %+v", run)
	}
	posted := led.credits()
	if len(posted) != 1 {
		t.Fatalf("the host ledger took %d credits, want 1", len(posted))
	}
	in := posted[0]
	if in.Org != "hanzo" || in.Subject != "hanzo/carol" || in.AmountCents != 2500 || in.IdempotencyKey != "sqpay_host" || !in.Cash || in.Test {
		t.Fatalf("the reload credited %+v, want 2500 cash to hanzo/carol under sqpay_host", in)
	}
	if bal, _ := led.Balance(ctx, "hanzo", "hanzo/carol", "usd", false); bal != 2500 {
		t.Fatalf("carol's host wallet holds %d, want 2500", bal)
	}
	if run := rechargeRun(t, ctx); run.Charged != 0 {
		t.Fatalf("a funded wallet was reloaded again: %+v", run)
	}

	// Spent below the threshold on the host ledger: a new trigger.
	if _, _, err := led.Debit(ctx, creditledger.DebitInput{Org: "hanzo", Subject: "hanzo/carol", Currency: "usd", Ref: "use-1", AmountCents: 2000}); err != nil {
		t.Fatal(err)
	}
	m.chargeRef = "sqpay_host_2"
	if run := rechargeRun(t, ctx); run.Charged != 1 || m.chargeCalls != 2 || len(led.credits()) != 2 {
		t.Fatalf("a wallet drained on the host was not reloaded: %+v (card calls %d, credits %d)", run, m.chargeCalls, len(led.credits()))
	}
}

// ─── every purchase arms ──────────────────────────────────────────────────

// subscribeAs buys with body as the given headers and answers the receipt.
func subscribeAs(t *testing.T, ctx ae.Context, org *organization.Organization, body string, headers map[string]string) map[string]any {
	t.Helper()
	resp := invokeSubscribeCard(org, ctx, body, headers)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("subscribe %s: status=%d body=%s, want 201", body, resp.StatusCode, bodyOf(resp))
	}
	return jsonBody(t, resp)
}

// reloadState is a receipt's reload state, "" when the receipt carries none.
func reloadState(out map[string]any) string {
	r, ok := out["reload"].(map[string]any)
	if !ok {
		return ""
	}
	s, _ := r["state"].(string)
	return s
}

// armedAt reports whether payer's rule is on at the plan's reload figures.
func armedAt(t *testing.T, ctx ae.Context, org *organization.Organization, payer string) bool {
	t.Helper()
	c := loadAutoRecharge(datastore.New(org.Namespaced(ctx)), payer)
	return c != nil && c.Enabled && c.ThresholdCents == ultraReload.ThresholdCents && c.AmountCents == ultraReload.AmountCents
}

// A new card and a saved card both pay the first period and arm the rule, and the
// receipt says so.
func TestACardPurchaseArmsTheReload(t *testing.T) {
	for name, body := range map[string]func(t *testing.T, ctx ae.Context, org *organization.Organization) string{
		"new card": func(*testing.T, ae.Context, *organization.Organization) string {
			return `{"sourceId":"cnon:ok","planId":"ultra"}`
		},
		"saved card": func(t *testing.T, ctx ae.Context, org *organization.Organization) string {
			pm := seedSavedCard(t, datastore.New(org.Namespaced(ctx)), org.Name, "ccof_saved", "cust_saved")
			return `{"paymentMethodId":"` + pm.Id() + `","planId":"ultra"}`
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := ae.NewContext()
			defer ctx.Close()
			zero := 0
			reloadPlan(t, ctx, "ultra", 19500, &zero, ultraReload)
			org := moneyOrg("arm-" + strings.ReplaceAll(name, " ", "-"))
			withFakeSquare(t, squareMock("cust_c", "ccof_c", "sqpay_c"))

			out := subscribeAs(t, ctx, org, body(t, ctx, org), nil)
			if got := reloadState(out); got != armed {
				t.Fatalf("receipt reload state %q, want %q", got, armed)
			}
			if !armedAt(t, ctx, org, org.Name) {
				t.Fatal("the rule is not on at the plan's figures")
			}
		})
	}
}

// A person who pays from a wallet of their own arms their own rule, charging their
// own card; the org's own account is untouched.
func TestAPersonalPurchaseArmsThePersonsRule(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	reloadPlan(t, ctx, "ultra", 19500, nil, ultraReload)
	org := moneyOrg("hanzo")
	withFakeSquare(t, squareMock("cust_d", "ccof_d", "sqpay_d"))

	out := subscribeAs(t, ctx, org, `{"sourceId":"cnon:ok","planId":"ultra"}`,
		map[string]string{"X-User-Id": "dana", "X-User-IsOrgAdmin": "false"})
	if got := reloadState(out); got != armed {
		t.Fatalf("receipt reload state %q, want %q", got, armed)
	}
	if !armedAt(t, ctx, org, "hanzo/dana") {
		t.Fatal("dana's own rule is not on")
	}
	if c := loadAutoRecharge(datastore.New(org.Namespaced(ctx)), "hanzo"); c != nil {
		t.Fatalf("a person's purchase armed the org's own account: %+v", c)
	}
}

// A wallet pays the first period and keeps no card, so the reload — which charges
// the card on file — arms only when one is there.
func TestAWalletPurchaseArmsWhenACardIsOnFile(t *testing.T) {
	for _, card := range []bool{false, true} {
		ctx := ae.NewContext()
		reloadPlan(t, ctx, "ultra", 19500, nil, ultraReload)
		name := "arm-wallet"
		if card {
			name = "arm-wallet-card"
		}
		org := moneyOrg(name)
		if card {
			seedSavedCard(t, datastore.New(org.Namespaced(ctx)), name, "ccof_w", "cust_w")
		}
		withFakeSquare(t, walletMock("fp_wallet_"+name))

		out := subscribeAs(t, ctx, org, walletBody("wnon:apple", "ultra", "", ""), nil)
		want := needsCard
		if card {
			want = armed
		}
		if got := reloadState(out); got != want {
			t.Errorf("card on file %v: receipt reload state %q, want %q", card, got, want)
		}
		if got := armedAt(t, ctx, org, name); got != card {
			t.Errorf("card on file %v: rule armed %v", card, got)
		}
		ctx.Close()
	}
}

// A trial opened before commerce stopped selling them ends at its end: the
// renewal sweep charges nothing, ends the row, and switches no reload on, even on
// a plan that states one and with a card on the row.
func TestAnOpenTrialEndsUnchargedAndArmsNothing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	reloadPlan(t, ctx, "ultra", 19500, nil, ultraReload)
	org := moneyOrg("arm-trial")
	m := squareMock("cust_tr", "ccof_tr", "sqpay_tr")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, org.Name, "ultra", "ccof_tr", "cust_tr")
	row := reload(t, db, sub.Id())
	row.Status = subscription.Trialing
	row.TrialStart, row.TrialEnd = time.Now().AddDate(0, 0, -8), time.Now().AddDate(0, 0, -1)
	if err := row.Update(); err != nil {
		t.Fatal(err)
	}
	out := renewDue(ctx, org, "")
	if len(out) != 1 || !out[0].Lapsed || m.chargeCalls != 0 {
		t.Fatalf("the trial's end: %+v, %d charge(s); want it lapsed and nothing charged", out, m.chargeCalls)
	}
	if got := reload(t, db, sub.Id()); got.Status != subscription.Canceled {
		t.Fatalf("after its end the trial is %s, want canceled", got.Status)
	}
	if armedAt(t, ctx, org, org.Name) {
		t.Fatal("a trial's end switched the reload on")
	}
}

// A holder still inside a card trial opened before commerce stopped selling them
// buys the plan like anyone else: the sale charges now, and the trial it replaces
// ends once the payment settles.
func TestATrialHolderBuysThePlan(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := moneyOrg("trial-buyer")
	m := squareMock("cust_tb", "ccof_tb", "sqpay_tb")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, org.Name, "go", "ccof_tb", "cust_tb")
	row := reload(t, db, sub.Id())
	row.Status = subscription.Trialing
	row.TrialStart, row.TrialEnd = time.Now().AddDate(0, 0, -2), time.Now().AddDate(0, 0, 5)
	if err := row.Update(); err != nil {
		t.Fatal(err)
	}
	r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"go"}`, nil)
	if r.StatusCode != http.StatusCreated || m.chargeCalls != 1 || m.lastChargeAmount != 800 {
		t.Fatalf("a trial holder buying go: %d %s, charged %d time(s) at %d; want 201 at 800", r.StatusCode, bodyOf(r), m.chargeCalls, m.lastChargeAmount)
	}
	if got := reload(t, db, sub.Id()); got.Status != subscription.Canceled {
		t.Fatalf("the trial the sale replaced is %s, want canceled", got.Status)
	}
}
