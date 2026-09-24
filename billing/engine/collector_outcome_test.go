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

// TestCollectInvoice_AnUnresolvedAttemptIsResentAsSent — the provider answers a key
// only for the request it first saw. While an attempt's outcome is unknown the key is
// kept, so the next attempt must ask for the amount the first asked for, whatever the
// prepaid balance holds by then; prepaid covers only what that amount leaves owed. A
// definite answer clears the pin and the next attempt is computed afresh.
func TestCollectInvoice_AnUnresolvedAttemptIsResentAsSent(t *testing.T) {
	pending := &processor.Decline{Processor: processor.Square, Category: processor.StatusCategory, Code: "PENDING"}
	refused := &processor.Decline{Processor: processor.Square, Category: "PAYMENT_METHOD_ERROR", Code: "CARD_DECLINED"}
	for _, tc := range []struct {
		name     string
		balances []int64 // the prepaid balance before each attempt
		answers  []error // the provider's answer to each attempt; nil settles
		asked    []int64 // what each attempt asked the card for
		paid     bool
		drawn    int64
	}{
		{"the balance grew while the payment settled", []int64{0, 100}, []error{pending, nil}, []int64{1900, 1900}, true, 0},
		{"the balance shrank while the payment settled", []int64{400, 0}, []error{pending, nil}, []int64{1500, 1500}, false, 0},
		{"prepaid still covers the part the card was not asked for", []int64{400, 1000}, []error{pending, nil}, []int64{1500, 1500}, true, 400},
		{"it stays pinned across unknown answers", []int64{0, 100, 300}, []error{pending, pending, nil}, []int64{1900, 1900, 1900}, true, 0},
		{"a refusal releases the amount", []int64{0, 0, 100}, []error{pending, refused, nil}, []int64{1900, 1900, 1800}, true, 100},
	} {
		inv := &billinginvoice.BillingInvoice{}
		inv.Status = billinginvoice.Open
		inv.AmountDue = 1900
		inv.SubscriptionId = "sub_pinned"
		pre := &purse{}
		var asked []int64
		for i, answer := range tc.answers {
			pre.balance = tc.balances[i]
			charger := ProviderCharger(func(_ context.Context, _ *datastore.Datastore, _ *billinginvoice.BillingInvoice, cents int64) (string, error) {
				asked = append(asked, cents)
				if answer != nil {
					return "", answer
				}
				return "sqpay_1", nil
			})
			if _, err := CollectInvoice(nil, nil, inv, pre, charger); err != nil {
				t.Fatalf("%s: attempt %d: %v", tc.name, i+1, err)
			}
		}
		if fmt.Sprint(asked) != fmt.Sprint(tc.asked) {
			t.Errorf("%s: the card was asked for %v, want %v", tc.name, asked, tc.asked)
		}
		if got := inv.Status == billinginvoice.Paid; got != tc.paid {
			t.Errorf("%s: invoice %s, want paid=%v", tc.name, inv.Status, tc.paid)
		}
		if drawn := sum(pre.draws); drawn != tc.drawn {
			t.Errorf("%s: prepaid drew %d, want %d", tc.name, drawn, tc.drawn)
		}
		if _, pinned := pinnedCard(inv); pinned {
			t.Errorf("%s: the pin outlived a definite answer: %v", tc.name, inv.Metadata)
		}
	}
}

func sum(xs []int64) (n int64) {
	for _, x := range xs {
		n += x
	}
	return n
}
