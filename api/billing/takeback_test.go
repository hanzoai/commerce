package billing

import (
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A dispute that withholds the funds of the SALE that switched a reload on (a stolen
// card buying Ultra) stops that reload, as a disputed reload payment does; a refund of
// that sale does too.
func TestADisputedSaleStopsTheReloadItArmed(t *testing.T) {
	const secret = "whsec_sale_dispute"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	reloadPlan(t, ctx, "ultra", 19500, nil, ultraReload)

	for _, c := range []struct {
		org, ref string
		event    []byte
	}{
		{"sale-dp", "sqpay_sale_dp", disputeEvent("evt_sale_dp_1", "dispute.created", "dp_sale", "sqpay_sale_dp", "LOST", 19500)},
		{"sale-rf", "sqpay_sale_rf", refundEvent("evt_sale_rf_1", "refund.updated", "sqrf_sale", "sqpay_sale_rf", "COMPLETED", 19500)},
	} {
		org := webhookOrg(t, ctx, c.org, true)
		withFakeSquare(t, squareMock("cust_"+c.org, "ccof_"+c.org, c.ref))
		out := subscribeAs(t, ctx, org, `{"sourceId":"cnon:ok","planId":"ultra"}`, nil)
		if got := reloadState(out); got != armed || !armedAt(t, ctx, org, org.Name) {
			t.Fatalf("%s: the sale did not switch the reload on (%q)", c.org, got)
		}
		deliver(t, ctx, secret, c.event)
		if armedAt(t, ctx, org, org.Name) {
			t.Fatalf("%s: the sale's payment was taken back and the reload it switched on is still on", c.org)
		}
	}
}

// A saved-card top-up records which wallet its payment funded, as the token top-up
// does, so a refund of it takes the money back from that wallet.
func TestARefundedTopUpIsTakenBackFromItsWallet(t *testing.T) {
	const secret = "whsec_topup_refund"
	registerSquare(t, secret)
	led := newFakeLedger()
	injectLedger(t, led)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("topup-rf")
	pm := seedSavedCard(t, datastore.New(org.Namespaced(ctx)), org.Name, "ccof_trf", "cust_trf")
	withFakeSquare(t, squareMock("", "", "sqpay_topup_rf"))

	if got, err := TopupCard(ctx, org, TopupCardIn{MethodID: pm.Id(), AmountCents: 2500, Subject: org.Name, IdempotencyKey: "topup-rf"}); err != nil || got.Status != "ok" {
		t.Fatalf("top-up: %+v, %v", got, err)
	}
	where, found, err := paymentorg.Get("sqpay_topup_rf")
	if err != nil || !found || where.Org != org.Name || where.Subject != org.Name || where.Invoice != "" {
		t.Fatalf("the top-up's payment is indexed as %+v (found %v, %v), want %s's wallet", where, found, err, org.Name)
	}

	deliver(t, ctx, secret, refundEvent("evt_topup_rf_1", "refund.updated", "sqrf_topup", "sqpay_topup_rf", "COMPLETED", 2500))
	if len(led.clawbacks) != 1 || led.clawbacks[0].Subject != org.Name || led.clawbacks[0].Payment != "sqpay_topup_rf" ||
		led.clawbacks[0].AmountCents != 2500 {
		t.Fatalf("clawbacks %+v, want the top-up's 2500 taken back from %s", led.clawbacks, org.Name)
	}
}
