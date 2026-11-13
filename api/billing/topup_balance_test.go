package billing

import (
	"encoding/json"
	"strings"
	"testing"
)

// A balance is read from the ledger the spend gate reads, after the host has
// credited it, so only the host states one. Commerce's own per-subject table is a
// different book (it reported 4000 where /v1/billing/balance said 3500), and a
// top-up answer that carries its number is a second source of truth.
func TestATopupAnswerStatesNoBalance(t *testing.T) {
	for name, v := range map[string]any{
		"TakePaymentOut": TakePaymentOut{TransactionID: "t", Status: "ok", ProcessorRef: "ch_1"},
		"TopupCardOut":   TopupCardOut{TransactionID: "t", Status: "ok"},
		"receipt":        receipt{TransactionId: "t", ProcessorRef: "ch_1", AmountCents: 500},
	} {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(string(b)), "balance") {
			t.Errorf("%s answers a balance: %s", name, b)
		}
	}
}
