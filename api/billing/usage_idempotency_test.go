package billing

import (
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// floorCents is whole cents in a micro-dollar total, toward -inf, so the span
// arithmetic in usageCents stays exact even across a compensating subtraction.
func TestFloorCents(t *testing.T) {
	for micros, cents := range map[int64]int64{0: 0, 1: 0, 9999: 0, 10000: 1, 19999: 1, 1000000: 100, -1: -1, -10000: -1, -10001: -2} {
		if got := floorCents(micros); got != cents {
			t.Errorf("floorCents(%d) = %d, want %d", micros, got, cents)
		}
	}
}

// TestRecordUsage_Idempotent_NoDoubleDebit proves the money-critical invariant
// added to RecordUsage: a retry/double-submit of the SAME usage (same
// requestId / X-Idempotency-Key) creates AT MOST ONE withdraw. Mirrors
// TestTopupWithToken_ClientKeyRetry_OneCharge (the proven credit-side guard).
//
// Chat's spendTokens fires per-completion and is retried on stream/abort, so
// without this guard every retry would double-debit the ledger.
func TestRecordUsage_Idempotent_NoDoubleDebit(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := &organization.Organization{}
	org.Name = "usage-idem-org"
	org.Live = true
	db := datastore.New(org.Namespaced(ctx))

	const subject = "usage-idem-org/alice@example.com"
	seedBalance(t, ctx, org, subject, 1000) // seed $10.00

	scope := "billing-usage"
	const key = "req-abc-123" // the requestId chat sends per spend

	debitOnce := func() {
		rec, replay, err := idempotencykey.Begin(db, scope, key)
		if err != nil {
			t.Fatalf("Begin: %v", err)
		}
		if replay {
			// A completed guard: the debit already happened — do NOT withdraw again.
			if rec.Status != idempotencykey.StatusCompleted {
				t.Fatalf("replay but status=%q, want completed", rec.Status)
			}
			return
		}
		trans := transaction.New(db)
		trans.Type = transaction.Withdraw
		trans.SourceId = subject
		trans.SourceKind = "iam-user"
		trans.Currency = currency.USD
		trans.Amount = currency.Cents(100) // $1.00 debit
		trans.Tags = "api-usage"
		if err := trans.Create(); err != nil {
			t.Fatalf("withdraw Create: %v", err)
		}
		if err := idempotencykey.Complete(rec, `{"type":"withdraw","amount":100}`); err != nil {
			t.Fatalf("Complete: %v", err)
		}
	}

	debitOnce() // first spend
	debitOnce() // retry with the SAME requestId — must be a no-op replay

	// Exactly ONE $1 debit: 1000 - 100 = 900. Two would leave 800.
	if got := balanceOf(t, ctx, org, subject); got != 900 {
		t.Fatalf("balance = %d, want 900 — DOUBLE-DEBIT: idempotent retry withdrew twice", got)
	}
}
