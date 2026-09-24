package billing

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestRefusal_OnlyARefusalRotatesTheGatewayKey — the gateway answers a key the way it
// answered it first, so a refusal must move the next attempt onto a key of its own,
// and a failure that was not a refusal must not: a lost response retried under a new
// key is the double charge the key exists to prevent.
func TestRefusal_OnlyARefusalRotatesTheGatewayKey(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("rotate-co")

	m := squareMock("cust_k", "ccof_k", "sqpay_k")
	m.chargeErr = errors.New("gateway timeout")
	withFakeSquare(t, m)
	in := TakePaymentIn{SourceID: "cnon:a", AmountCents: 500, Subject: "rotate-co"}
	if _, f := TakePayment(ctx, org, in); f == nil || !IsProcessorFailed(f.Err) {
		t.Fatalf("precondition: the first attempt was not a processor failure: %+v", f)
	}
	in.SourceID = "cnon:b"
	_, _ = TakePayment(ctx, org, in)
	if keys := chargedKeysOf(m); len(keys) != 1 {
		t.Errorf("a retry after a failure that was not a refusal reached the gateway under %d keys (%v), want 1", len(keys), keys)
	}

	m2 := squareMock("cust_k2", "ccof_k2", "sqpay_k2")
	m2.chargeErr = cardDeclined
	withFakeSquare(t, m2)
	in = TakePaymentIn{SourceID: "cnon:c", AmountCents: 700, Subject: "rotate-co"}
	if _, f := TakePayment(ctx, org, in); f == nil || f.Status != 402 {
		t.Fatalf("precondition: the first attempt was not declined: %+v", f)
	}
	m2.chargeErr = nil
	in.SourceID = "cnon:d"
	if _, f := TakePayment(ctx, org, in); f != nil {
		t.Fatalf("the retry after a refusal answered %+v", f)
	}
	if keys := chargedKeysOf(m2); len(keys) != 2 {
		t.Errorf("a retry after a refusal reached the gateway under %d keys (%v), want 2", len(keys), keys)
	}
}

// TestRefusal_AWalletAtTheCeilingIsNotTriedAgain — five refusals in the window and
// the next card is refused before the processor is asked, on every card path. Another
// wallet in the same org is untouched.
func TestRefusal_AWalletAtTheCeilingIsNotTriedAgain(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("ceiling-co")
	m := squareMock("cust_c", "ccof_c", "sqpay_c")
	m.chargeErr = cardDeclined
	withFakeSquare(t, m)

	for i := range declineCeiling {
		_, f := TakePayment(ctx, org, TakePaymentIn{SourceID: fmt.Sprintf("cnon:%d", i), AmountCents: 500 + int64(i), Subject: "ceiling-co/mallory"})
		if f == nil || f.Status != 402 {
			t.Fatalf("refusal %d answered %+v", i, f)
		}
	}
	calls := m.chargeCalls

	_, f := TakePayment(ctx, org, TakePaymentIn{SourceID: "cnon:next", AmountCents: 900, Subject: "ceiling-co/mallory"})
	if f == nil || f.Status != 429 || !IsDeclineCeiling(f.Err) {
		t.Fatalf("the sixth attempt answered %+v, want 429 at the ceiling", f)
	}
	_, err := SubscribeCard(ctx, org, SubscribeIn{SourceID: "cnon:sale", PlanID: "dev", Subject: "ceiling-co/mallory"})
	if !IsDeclineCeiling(err) {
		t.Errorf("a plan sale at the ceiling answered %v", err)
	}
	if m.chargeCalls != calls {
		t.Errorf("the processor was asked %d more time(s) after the ceiling", m.chargeCalls-calls)
	}

	m.chargeErr = nil
	if _, f := TakePayment(ctx, org, TakePaymentIn{SourceID: "cnon:alice", AmountCents: 500, Subject: "ceiling-co/alice"}); f != nil {
		t.Errorf("another wallet in the org was held by mallory's refusals: %+v", f)
	}
}

// TestRefusal_AResultThatDidNotSettleNeverEchoesTheProcessor — a processor that
// answers a charge it did not take, with its own words beside it, is a refusal read
// by its Decline, or a plain decline when it states none. The words never reach the
// buyer.
func TestRefusal_AResultThatDidNotSettleNeverEchoesTheProcessor(t *testing.T) {
	for _, tc := range []struct {
		name string
		res  *processor.PaymentResult
		want string
	}{
		{"square's status refusal", &processor.PaymentResult{
			ErrorMessage: "payment not completed (status: PENDING)",
			Error:        &processor.Decline{Processor: processor.Square, Category: "PAYMENT_STATUS", Code: "PENDING"},
		}, "Your card was declined by the bank."},
		{"a refusal with only words", &processor.PaymentResult{ErrorMessage: "cvv mismatch; card 4111 1111 1111 1111 expired"}, "Your card was declined by the bank."},
	} {
		if got := parseCardDeclineReason(tc.res, nil); got != tc.want {
			t.Errorf("%s: the buyer reads %q, want %q", tc.name, got, tc.want)
		}
	}
	if got := parseCardDeclineReason(nil, errors.New("dial tcp: connection refused")); got != processorSentence {
		t.Errorf("a processor that did not answer reads %q, want %q", got, processorSentence)
	}
}

// TestRefusal_ARenewalKeepsTheDeclineBeneathItsSentence — dunning records a refused
// renewal as the buyer's sentence and keeps the processor's Decline beneath it; a
// renewal the processor failed to answer is a processor failure, never a decline.
func TestRefusal_ARenewalKeepsTheDeclineBeneathItsSentence(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("renew-code")
	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, "renew-code", "dev", "ccof_rc", "cust_rc")
	inv := seedOpenInvoice(t, db, sub, 1900)

	m := squareMock("", "", "")
	m.chargeErr = &processor.Decline{Processor: processor.Square, Category: "PAYMENT_METHOD_ERROR", Code: "INSUFFICIENT_FUNDS"}
	withFakeSquare(t, m)
	_, err := chargeProviderForOrg(org)(ctx, db, inv, 1900)
	if d, ok := DeclineOf(err); !ok || d.Code != "INSUFFICIENT_FUNDS" || err.Error() != "Insufficient funds." {
		t.Errorf("a refused renewal answered %v", err)
	}

	m.chargeErr = errors.New("gateway timeout")
	inv.AttemptCount++
	if _, err := chargeProviderForOrg(org)(ctx, db, inv, 1900); !IsProcessorFailed(err) {
		t.Errorf("a renewal the processor failed to answer answered %v", err)
	}
}

// TestRefusal_AVaultTheProcessorFailedIsNotADeclinedSale — a card the processor could
// not vault for reasons of its own is a processor failure, not the card's refusal.
func TestRefusal_AVaultTheProcessorFailedIsNotADeclinedSale(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	m := squareMock("cust_v", "ccof_v", "sqpay_v")
	m.addCardErr = processor.NewPaymentError(processor.Square, "ACCESS_TOKEN_EXPIRED", "square answered 401 AUTHENTICATION_ERROR ACCESS_TOKEN_EXPIRED", nil)
	withFakeSquare(t, m)
	_, err := SubscribeCard(ctx, moneyOrg("vault-fail"), SubscribeIn{SourceID: "cnon:v", PlanID: "dev", Subject: "vault-fail"})
	if !IsProcessorFailed(err) || IsSaleDeclined(err) {
		t.Errorf("a failed vault answered %v (declined sale %v)", err, IsSaleDeclined(err))
	}
}
