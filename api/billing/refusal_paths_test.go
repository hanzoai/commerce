package billing

import (
	"context"
	"fmt"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// declineOn is a Square refusal with the given code.
func declineOn(code string) *processor.Decline {
	return &processor.Decline{Processor: processor.Square, Category: "PAYMENT_METHOD_ERROR", Code: code}
}

// unanswered is a Square failure that is not a refusal: the processor never looked
// at the card.
var unanswered = processor.NewPaymentError(processor.Square, "ACCESS_TOKEN_EXPIRED", "square answered 401 AUTHENTICATION_ERROR ACCESS_TOKEN_EXPIRED", nil)

// saveCardAs saves a fresh card for subject through CreateMethod, the card-save path.
func saveCardAs(ctx context.Context, org string, subject, nonce string) error {
	_, _, err := CreateMethod(ctx, moneyOrg(org), "buyer@test", nil, CreateMethodIn{CustomerId: subject, Type: "card", ProviderRef: nonce})
	return err
}

// refusalsOf is the wallet's refusal count for this window.
func refusalsOf(ctx context.Context, org, subject string) int {
	return count(datastore.New(moneyOrg(org).Namespaced(ctx)), walletScope+subject, windowOf())
}

// TestRefusal_CardSaveIsHeldAndCountedLikeACharge — vaulting a card validates it, so a
// card save is a card attempt: a refusal is counted against the wallet, a wallet at
// its ceiling has no card vaulted, and a processor that failed to answer is a
// processor failure that counts nothing.
func TestRefusal_CardSaveIsHeldAndCountedLikeACharge(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	m := squareMock("cust_s", "ccof_s", "")
	m.addCardErr = declineOn("CVV_FAILURE")
	withFakeSquare(t, m)

	err := saveCardAs(ctx, "save-co", "save-co", "cnon:0")
	if !IsCardDeclined(err) || err.Error() != "The security code (CVV) didn't match." {
		t.Fatalf("a refused card save answered %v", err)
	}
	if n := refusalsOf(ctx, "save-co", "save-co"); n != 1 {
		t.Fatalf("a refused card save counted %d refusals, want 1", n)
	}
	for i := 1; i < declineCeiling; i++ {
		_ = saveCardAs(ctx, "save-co", "save-co", fmt.Sprintf("cnon:%d", i))
	}
	m.addCardErr = nil
	if err := saveCardAs(ctx, "save-co", "save-co", "cnon:good"); !IsDeclineCeiling(err) {
		t.Errorf("a card save at the ceiling answered %v", err)
	}

	m.addCardErr = unanswered
	err = saveCardAs(ctx, "save-co2", "save-co2", "cnon:x")
	if !IsProcessorFailed(err) || IsCardDeclined(err) {
		t.Errorf("a vault the processor failed answered %v (declined %v)", err, IsCardDeclined(err))
	}
	if n := refusalsOf(ctx, "save-co2", "save-co2"); n != 0 {
		t.Errorf("a processor failure on card save counted %d refusals", n)
	}
}

// TestRefusal_TheSavedCardTopUpIsHeldAndCounted — the buyer's saved-card top-up is a
// card attempt: a refusal counts, the ceiling holds, and a processor failure is
// neither a decline nor a refusal.
func TestRefusal_TheSavedCardTopUpIsHeldAndCounted(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("saved-co")
	pm := seedSavedCard(t, datastore.New(org.Namespaced(ctx)), "saved-co", "ccof_sv", "cust_sv")
	m := squareMock("cust_sv", "ccof_sv", "sqpay_sv")
	withFakeSquare(t, m)
	topup := func(i int) error {
		_, err := TopupCard(ctx, org, TopupCardIn{MethodID: pm.Id(), AmountCents: 500 + int64(i), Subject: "saved-co"})
		return err
	}

	m.chargeErr = unanswered
	if err := topup(100); !IsProcessorFailed(err) {
		t.Fatalf("a saved-card charge the processor failed answered %v", err)
	}
	if _, ok := DeclineOf(topup(101)); ok {
		t.Error("a processor failure on the saved-card top-up read as a decline")
	}
	if n := refusalsOf(ctx, "saved-co", "saved-co"); n != 0 {
		t.Fatalf("processor failures counted %d refusals", n)
	}

	m.chargeErr = declineOn("CARD_DECLINED")
	for i := range declineCeiling {
		if _, ok := DeclineOf(topup(i)); !ok {
			t.Fatalf("refusal %d was not a decline", i)
		}
	}
	if n := refusalsOf(ctx, "saved-co", "saved-co"); n != declineCeiling {
		t.Fatalf("%d refusals counted, want %d", n, declineCeiling)
	}
	calls := m.chargeCalls
	if err := topup(99); !IsDeclineCeiling(err) {
		t.Errorf("a saved-card top-up at the ceiling answered %v", err)
	}
	if m.chargeCalls != calls {
		t.Error("the card was tried after the ceiling")
	}
}

// TestRefusal_APlanSaleVaultRefusalIsCounted — the card a fresh-card sale vaults is
// validated there, so a refusal at the vault counts against the wallet like a refused
// charge.
func TestRefusal_APlanSaleVaultRefusalIsCounted(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	m := squareMock("cust_pv", "ccof_pv", "sqpay_pv")
	m.addCardErr = declineOn("VERIFY_AVS_FAILURE")
	withFakeSquare(t, m)
	_, err := SubscribeCard(ctx, moneyOrg("sale-vault"), SubscribeIn{SourceID: "cnon:v", PlanID: "dev", Subject: "sale-vault"})
	if !IsSaleDeclined(err) {
		t.Fatalf("a refused vault answered %v", err)
	}
	if n := refusalsOf(ctx, "sale-vault", "sale-vault"); n != 1 {
		t.Errorf("a refused sale vault counted %d refusals, want 1", n)
	}
}

// TestRefusal_OneCardAttemptPerWalletAtATime — an attempt the wallet already has in
// progress holds the next one, on every buyer path, so no run of parallel attempts
// reaches the processor past the ceiling.
func TestRefusal_OneCardAttemptPerWalletAtATime(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("flight-co")
	db := datastore.New(org.Namespaced(ctx))
	pm := seedSavedCard(t, db, "flight-co", "ccof_f", "cust_f")
	m := squareMock("cust_f", "ccof_f", "sqpay_f")
	withFakeSquare(t, m)

	release, err := attempt(db, org.Name, "flight-co")
	if err != nil {
		t.Fatal(err)
	}
	if _, f := TakePayment(ctx, org, TakePaymentIn{SourceID: "cnon:t", AmountCents: 500, Subject: "flight-co"}); f == nil || f.Status != 409 || !IsAttemptInFlight(f.Err) {
		t.Errorf("a token top-up during another attempt answered %+v", f)
	}
	if _, err := TopupCard(ctx, org, TopupCardIn{MethodID: pm.Id(), AmountCents: 500, Subject: "flight-co"}); !IsAttemptInFlight(err) {
		t.Errorf("a saved-card top-up during another attempt answered %v", err)
	}
	if _, err := SubscribeCard(ctx, org, SubscribeIn{SourceID: "cnon:s", PlanID: "dev", Subject: "flight-co"}); !IsAttemptInFlight(err) {
		t.Errorf("a plan sale during another attempt answered %v", err)
	}
	if err := saveCardAs(ctx, "flight-co", "flight-co", "cnon:c"); !IsAttemptInFlight(err) {
		t.Errorf("a card save during another attempt answered %v", err)
	}
	if _, f := TakePayment(ctx, org, TakePaymentIn{SourceID: "cnon:o", AmountCents: 500, Subject: "flight-co/other"}); f != nil {
		t.Errorf("another wallet in the org was held: %+v", f)
	}
	if m.chargeCalls != 1 {
		t.Errorf("the processor was asked %d times, want once (the other wallet)", m.chargeCalls)
	}
	release()
	if _, f := TakePayment(ctx, org, TakePaymentIn{SourceID: "cnon:after", AmountCents: 600, Subject: "flight-co"}); f != nil {
		t.Errorf("after the attempt ended the wallet answered %+v", f)
	}
}

// TestRefusal_APendingPaymentIsProcessingNotADecline — a payment the processor took
// and has not settled answers 409 "still processing", counts nothing against the
// wallet, and leaves the next attempt on the same gateway key; a sale in that state is
// held, not declined.
func TestRefusal_APendingPaymentIsProcessingNotADecline(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	pending := &processor.Decline{Processor: processor.Square, Category: processor.StatusCategory, Code: "PENDING"}
	m := squareMock("cust_pp", "ccof_pp", "sqpay_pp")
	m.chargeErr = pending
	withFakeSquare(t, m)

	org := moneyOrg("pending-co")
	_, f := TakePayment(ctx, org, TakePaymentIn{SourceID: "cnon:a", AmountCents: 500, Subject: "pending-co"})
	if f == nil || f.Status != 409 || f.Message != "Your payment is still processing. Please don't pay again." {
		t.Fatalf("a pending payment answered %+v", f)
	}
	_, _ = TakePayment(ctx, org, TakePaymentIn{SourceID: "cnon:b", AmountCents: 500, Subject: "pending-co"})
	if keys := chargedKeysOf(m); len(keys) != 1 {
		t.Errorf("a retry of a pending payment reached the processor under %d keys, want 1", len(keys))
	}
	if n := refusalsOf(ctx, "pending-co", "pending-co"); n != 0 {
		t.Errorf("a pending payment counted %d refusals", n)
	}

	_, err := SubscribeCard(ctx, moneyOrg("pending-sale"), SubscribeIn{SourceID: "cnon:s", PlanID: "dev", Subject: "pending-sale"})
	if !IsSaleConflict(err) || IsSaleDeclined(err) {
		t.Errorf("a sale whose payment is pending answered %v (conflict %v, declined %v)", err, IsSaleConflict(err), IsSaleDeclined(err))
	}
}
