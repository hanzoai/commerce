package engine

import (
	"context"
	"fmt"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/payment/processor"
)

// TestCollectInvoice_AnUnknownOutcomeKeepsTheAttemptsKey — the renewal's gateway key
// carries the invoice's attempt count, so advancing the count after an attempt whose
// outcome the provider did not state (it failed to answer, or the payment is still
// processing) sends the next attempt under a new key: the provider takes a second
// payment when the first may already have been taken. Such an attempt keeps the count;
// a refusal, which took nothing, advances it so dunning reaches the card again.
func TestCollectInvoice_AnUnknownOutcomeKeepsTheAttemptsKey(t *testing.T) {
	for _, tc := range []struct {
		name     string
		err      error
		advances bool
	}{
		{"the provider failed to answer", fmt.Errorf("the payment processor could not take the card: %w", processor.ErrUnknownOutcome), false},
		{"the payment is still processing", &processor.Decline{Processor: processor.Square, Category: processor.StatusCategory, Code: "PENDING"}, false},
		{"the card was refused", &processor.Decline{Processor: processor.Square, Category: "PAYMENT_METHOD_ERROR", Code: "CARD_DECLINED"}, true},
		{"a payment square failed", &processor.Decline{Processor: processor.Square, Category: processor.StatusCategory, Code: "FAILED"}, true},
	} {
		inv := &billinginvoice.BillingInvoice{}
		inv.Status = billinginvoice.Open
		inv.AmountDue = 1900
		inv.AttemptCount = 2
		charger := ProviderCharger(func(context.Context, *datastore.Datastore, *billinginvoice.BillingInvoice, int64) (string, error) {
			return "", tc.err
		})
		result, err := CollectInvoice(nil, nil, inv, nil, charger)
		if err != nil || result.Success {
			t.Fatalf("%s: collection answered %+v, %v", tc.name, result, err)
		}
		if got, want := inv.AttemptCount, map[bool]int{true: 3, false: 2}[tc.advances]; got != want {
			t.Errorf("%s: attempt count %d, want %d", tc.name, got, want)
		}
		if inv.Status != billinginvoice.Open || inv.LastAttemptAt.IsZero() {
			t.Errorf("%s: invoice %s, last attempt %v", tc.name, inv.Status, inv.LastAttemptAt)
		}
	}
}
