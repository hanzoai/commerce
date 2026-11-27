package billing

import (
	"strings"
	"testing"

	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/payment/processor"
)

func TestGetCryptoDeposit_StripeIntentReconcilesSucceededAndCanceled(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	led := newFakeLedger()
	injectLedger(t, led)

	// 1) A Stripe stablecoin deposit whose webhook was missed/delayed settles synchronously on poll.
	if err := pendingintent.PutQuoted("pi_crypto_poll_ok", org.Name, "buyer", pendingintent.Topup, 500, "usd", 0, "", 1, false); err != nil {
		t.Fatal(err)
	}
	if err := pendingintent.Bind("pi_crypto_poll_ok", pendingintent.Terms{Delayed: true, Network: "base"}); err != nil {
		t.Fatal(err)
	}
	useStripe(t, &stripeDouble{
		tx: &processor.Transaction{ID: "pi_crypto_poll_ok", ProcessorRef: "ch_crypto_poll_ok", Amount: 500, Currency: "usd", Status: "succeeded"},
	})

	dep, err := GetCryptoDeposit(ctx, org, "buyer", "pi_crypto_poll_ok")
	if err != nil {
		t.Fatalf("GetCryptoDeposit succeeded: %v", err)
	}
	if dep.Status != "succeeded" || dep.Chain != "base" || dep.AmountCents != 500 {
		t.Fatalf("deposit = %+v, want succeeded on base for 500", dep)
	}
	orgKey := strings.ToLower(org.Name)
	if got, _ := led.Balance(ctx, orgKey, "buyer", "usd", false); got != 500 {
		t.Fatalf("ledger balance = %d, want 500", got)
	}
	// Polling again must not double-credit.
	if _, err := GetCryptoDeposit(ctx, org, "buyer", "pi_crypto_poll_ok"); err != nil {
		t.Fatalf("second poll: %v", err)
	}
	if got, _ := led.Balance(ctx, orgKey, "buyer", "usd", false); got != 500 {
		t.Fatalf("ledger balance after second poll = %d, want 500", got)
	}

	// 2) A Stripe stablecoin deposit that was cancelled reports expired and settles the stale pending row.
	if err := pendingintent.PutQuoted("pi_crypto_poll_cancel", org.Name, "buyer", pendingintent.Topup, 5000, "usd", 0, "", 1, false); err != nil {
		t.Fatal(err)
	}
	if err := pendingintent.Bind("pi_crypto_poll_cancel", pendingintent.Terms{Delayed: true, Network: "base"}); err != nil {
		t.Fatal(err)
	}
	useStripe(t, &stripeDouble{
		tx: &processor.Transaction{ID: "pi_crypto_poll_cancel", Amount: 5000, Currency: "usd", Status: "canceled"},
	})

	dep, err = GetCryptoDeposit(ctx, org, "buyer", "pi_crypto_poll_cancel")
	if err != nil {
		t.Fatalf("GetCryptoDeposit canceled: %v", err)
	}
	if dep.Status != "expired" {
		t.Fatalf("canceled deposit status = %q, want expired", dep.Status)
	}
	if p, _, _ := pendingintent.Get("pi_crypto_poll_cancel"); p == nil || !p.Done {
		t.Fatalf("canceled pendingintent should be marked done, got %+v", p)
	}
	// Closed is not credited: every later read must still report expired.
	for range 3 {
		dep, err = GetCryptoDeposit(ctx, org, "buyer", "pi_crypto_poll_cancel")
		if err != nil || dep.Status != "expired" {
			t.Fatalf("repeat canceled poll = %+v (%v), want expired", dep, err)
		}
	}
	if got, _ := led.Balance(ctx, orgKey, "buyer", "usd", false); got != 500 {
		t.Fatalf("canceled deposit altered ledger: %d, want 500", got)
	}
	// A successful processor status with a closed row but no credit receipt
	// must not be reported as settled.
	if err := pendingintent.PutQuoted("pi_crypto_no_receipt", org.Name, "buyer", pendingintent.Topup, 5000, "usd", 0, "", 1, false); err != nil {
		t.Fatal(err)
	}
	if _, err := pendingintent.Settle("pi_crypto_no_receipt"); err != nil {
		t.Fatal(err)
	}
	useStripe(t, &stripeDouble{
		tx: &processor.Transaction{ID: "pi_crypto_no_receipt", ProcessorRef: "ch_no_receipt", Amount: 5000, Currency: "usd", Status: "succeeded"},
	})
	dep, err = GetCryptoDeposit(ctx, org, "buyer", "pi_crypto_no_receipt")
	if err != nil || dep.Status == "succeeded" {
		t.Fatalf("uncredited closed row = %+v (%v), must not report success", dep, err)
	}

}
