package billing

import (
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// Money a card pays into a prepaid balance lands in each ledger once: one deposit
// row in commerce's books under the payment's reference, and one credit in the
// host ledger the spend gate reads — whoever is the one that posts it — however
// many times the processor's payment webhook for the same payment is delivered
// afterwards, to whichever account it is booked against.

// depositsUnder counts the deposit rows in org's books stamped with the payment
// ref.
func depositsUnder(t *testing.T, ctx ae.Context, org, ref string) int {
	t.Helper()
	db := datastore.New(nscontext.WithNamespace(ctx, org))
	rows := make([]*transaction.Transaction, 0)
	if _, err := transaction.Query(db).Filter("SourceId=", ref).GetAll(&rows); err != nil {
		t.Fatalf("deposits under %s in %s: %v", ref, org, err)
	}
	n := 0
	for _, r := range rows {
		if r.Type == transaction.Deposit {
			n++
		}
	}
	return n
}

// creditsUnder counts the host ledger's credits posted under key.
func creditsUnder(led *fakeLedger, key string) int {
	n := 0
	for _, in := range led.credits() {
		if in.IdempotencyKey == key {
			n++
		}
	}
	return n
}

// gate is the payer's prepaid money as the spend gate reads it: through the host
// ledger when one is injected.
func gate(t *testing.T, ctx ae.Context, org *organization.Organization, subject string) int64 {
	t.Helper()
	n, err := prepaidFor(ctx, org).Available(ctx, subject, currency.USD)
	if err != nil {
		t.Fatalf("gate read for %s: %v", subject, err)
	}
	return n
}

// redeliver delivers the processor's settlement of payment ref for cents twice,
// under two event ids, booked once against the deployment's own account and once
// against the payer's org.
func redeliver(t *testing.T, ctx ae.Context, secret, org, ref, subject string, cents int64) {
	t.Helper()
	for i, booked := range []string{"", org} {
		body := settlementEvent("evt_once_"+ref+"_"+string(rune('a'+i)), ref, "COMPLETED", subject, cents)
		if r := deliverWebhook(ctx, booked, secret, body, ""); r.StatusCode != http.StatusOK {
			t.Fatalf("webhook for %s booked to %q: %d %s", ref, booked, r.StatusCode, bodyOf(r))
		}
	}
}

// An automatic reload: one deposit row under its reference, one host-ledger
// credit under the same reference (bank), and the payment webhook adds nothing.
func TestAReloadLandsOnceInBothLedgers(t *testing.T) {
	const secret = "whsec_once_reload"
	registerSquare(t, secret)
	led := newFakeLedger()
	injectLedger(t, led)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := payerOrg(t, ctx, "once-reload")
	payerRule(t, ctx, org, org.Name, 2500, 1000)
	withFakeSquare(t, squareMock("", "", "sqpay_once_reload"))

	if run := rechargeRun(t, ctx); run.Charged != 1 {
		t.Fatalf("the reload did not charge: %+v", run)
	}
	calls := led.calls
	redeliver(t, ctx, secret, org.Name, "sqpay_once_reload", org.Name, 2500)

	if n := depositsUnder(t, ctx, org.Name, "sqpay_once_reload"); n != 1 {
		t.Errorf("commerce holds %d deposit rows for the reload, want 1", n)
	}
	if n := depositsUnder(t, ctx, "hanzo", "sqpay_once_reload"); n != 0 {
		t.Errorf("the webhook wrote %d deposit rows in the deployment's books", n)
	}
	if n := creditsUnder(led, "sqpay_once_reload"); n != 1 || led.calls != calls {
		t.Errorf("the host ledger took %d credits under the reload's reference and %d more calls after the webhook, want 1 and 0", n, led.calls-calls)
	}
	if got := gate(t, ctx, org, org.Name); got != 2500 {
		t.Errorf("the gate reads %d, want the reload's 2500", got)
	}
}

// An organization's starting balance: one deposit row (BalanceID) and one host-ledger
// credit, both keyed on the sale's payment and both written by commerce (endow), as a
// reload's are; the sale's payment webhook, for the seats and the balance together,
// credits nothing. The host credits nothing of its own for it.
func TestAStartingBalanceLandsOnceInBothLedgers(t *testing.T) {
	const secret = "whsec_once_order"
	registerSquare(t, secret)
	led := newFakeLedger()
	injectLedger(t, led)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("once-order")
	withFakeSquare(t, squareMock("cust_oo", "ccof_oo", "sqpay_once_order"))
	orderRow(t, ctx, "once-ent", 20, 500)

	out := subscribeAs(t, ctx, org, `{"sourceId":"cnon:ok","planId":"once-ent","quantity":20,"interval":"year","balanceCents":40000}`, nil)
	balanceID, _ := out["balanceId"].(string)
	if balanceID == "" || out["amountCents"] != float64(520000) {
		t.Fatalf("receipt %v, want a balance id on one payment of 520000", out)
	}
	if led.calls != 1 || creditsUnder(led, "sqpay_once_order") != 1 {
		t.Fatalf("commerce posted %d host-ledger credits for the order (%d under the payment), want one under the payment", led.calls, creditsUnder(led, "sqpay_once_order"))
	}
	redeliver(t, ctx, secret, org.Name, "sqpay_once_order", org.Name, 520000)

	if n := depositsUnder(t, ctx, org.Name, "sqpay_once_order"); n != 1 {
		t.Errorf("commerce holds %d deposit rows for the starting balance, want 1", n)
	}
	if n := depositsUnder(t, ctx, "hanzo", "sqpay_once_order"); n != 0 {
		t.Errorf("the webhook wrote %d deposit rows in the deployment's books", n)
	}
	if led.calls != 1 || creditsUnder(led, "sqpay_once_order") != 1 {
		t.Errorf("the host ledger took %d calls (%d under the payment), want only commerce's one credit", led.calls, creditsUnder(led, "sqpay_once_order"))
	}
	if got := gate(t, ctx, org, org.Name); got != 40000 {
		t.Errorf("the gate reads %d, want the starting balance's 40000", got)
	}
}

// A saved-card top-up: one deposit row under its reference, and one credit in the host
// ledger, posted by commerce — the only writer — under the processor's reference. The
// payment webhook adds nothing to either.
func TestATopUpLandsOnceInBothLedgers(t *testing.T) {
	const secret = "whsec_once_topup"
	registerSquare(t, secret)
	led := newFakeLedger()
	injectLedger(t, led)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("once-topup")
	pm := seedSavedCard(t, datastore.New(org.Namespaced(ctx)), org.Name, "ccof_ot", "cust_ot")
	withFakeSquare(t, squareMock("", "", "sqpay_once_topup"))

	got, err := TopupCard(ctx, org, TopupCardIn{MethodID: pm.Id(), AmountCents: 2500, Subject: org.Name, IdempotencyKey: "topup-once"})
	if err != nil || got.Status != "ok" {
		t.Fatalf("top-up: %+v, %v", got, err)
	}
	if led.calls != 1 || creditsUnder(led, "sqpay_once_topup") != 1 {
		t.Fatalf("commerce posted %d host-ledger credits for the top-up (%d under the payment), want one", led.calls, creditsUnder(led, "sqpay_once_topup"))
	}
	redeliver(t, ctx, secret, org.Name, "sqpay_once_topup", org.Name, 2500)

	if n := depositsUnder(t, ctx, org.Name, "sqpay_once_topup"); n != 1 {
		t.Errorf("commerce holds %d deposit rows for the top-up, want 1", n)
	}
	if n := depositsUnder(t, ctx, "hanzo", "sqpay_once_topup"); n != 0 {
		t.Errorf("the webhook wrote %d deposit rows in the deployment's books", n)
	}
	if led.calls != 1 || creditsUnder(led, "sqpay_once_topup") != 1 {
		t.Errorf("the host ledger took %d calls (%d under the payment), want only commerce's one credit", led.calls, creditsUnder(led, "sqpay_once_topup"))
	}
	if got := gate(t, ctx, org, org.Name); got != 2500 {
		t.Errorf("the gate reads %d, want the top-up's 2500", got)
	}
}
