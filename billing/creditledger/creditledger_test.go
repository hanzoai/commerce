package creditledger

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func runParallel(n int, fn func(idx int)) {
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			fn(i)
		}()
	}
	close(start)
	wg.Wait()
}

func TestCreditLedger_ConcurrentDebitsNeverOverdraw(t *testing.T) {
	ctx := context.Background()
	led := NewMemCreditLedger()

	if _, bal, err := led.Credit(ctx, CreditInput{
		Org:            "acme",
		Subject:        "alice",
		Currency:       "usd",
		Reason:         "initial grant",
		IdempotencyKey: "seed-1000",
		AmountCents:    1000,
	}); err != nil || bal != 1000 {
		t.Fatalf("seed Credit: bal=%d err=%v, want 1000 nil", bal, err)
	}

	const workers = 50
	const debitAmt = 100
	var succeeded atomic.Int64
	var rejectedShort atomic.Int64
	errs := make([]error, workers)

	runParallel(workers, func(i int) {
		_, _, err := led.Debit(ctx, DebitInput{
			Org:         "acme",
			Subject:     "alice",
			Currency:    "usd",
			Reason:      "api usage",
			Ref:         fmt.Sprintf("call-%d", i),
			AmountCents: debitAmt,
		})
		errs[i] = err
		if err == nil {
			succeeded.Add(1)
		} else if errors.Is(err, ErrShort) {
			rejectedShort.Add(1)
		}
	})

	for i, err := range errs {
		if err != nil && !errors.Is(err, ErrShort) {
			t.Fatalf("worker %d returned unexpected error: %v", i, err)
		}
	}
	if succeeded.Load() != 10 {
		t.Fatalf("expected exactly 10 successful debits, got %d", succeeded.Load())
	}
	if rejectedShort.Load() != 40 {
		t.Fatalf("expected exactly 40 ErrShort refusals, got %d", rejectedShort.Load())
	}

	bal, err := led.Balance(ctx, "acme", "alice", "usd", false)
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if bal != 0 {
		t.Fatalf("final balance=%d, want 0 (never negative)", bal)
	}
	if sum := led.LedgerBalanceSum(); sum != 0 {
		t.Fatalf("double-entry sum=%d, want 0", sum)
	}
}

func TestCreditLedger_ConcurrentDuplicateIdempotencyKeys(t *testing.T) {
	ctx := context.Background()
	led := NewMemCreditLedger()

	const workers = 50
	creditIDs := make([]string, workers)
	creditErrs := make([]error, workers)

	runParallel(workers, func(i int) {
		id, _, err := led.Credit(ctx, CreditInput{
			Org:            "idem-org",
			Subject:        "",
			Currency:       "usd",
			Reason:         "welcome",
			IdempotencyKey: "credit-once",
			AmountCents:    500,
		})
		creditIDs[i] = id
		creditErrs[i] = err
	})

	for i := 0; i < workers; i++ {
		if creditErrs[i] != nil {
			t.Fatalf("credit worker %d error: %v", i, creditErrs[i])
		}
		if creditIDs[i] == "" || creditIDs[i] != creditIDs[0] {
			t.Fatalf("credit worker %d txID=%q != %q", i, creditIDs[i], creditIDs[0])
		}
	}

	bal, _ := led.Balance(ctx, "idem-org", "", "usd", false)
	if bal != 500 {
		t.Fatalf("balance after duplicate concurrent credits=%d, want 500", bal)
	}

	debitIDs := make([]string, workers)
	debitErrs := make([]error, workers)
	runParallel(workers, func(i int) {
		id, _, err := led.Debit(ctx, DebitInput{
			Org:         "idem-org",
			Subject:     "",
			Currency:    "usd",
			Reason:      "charge",
			Ref:         "debit-once",
			AmountCents: 300,
		})
		debitIDs[i] = id
		debitErrs[i] = err
	})

	for i := 0; i < workers; i++ {
		if debitErrs[i] != nil {
			t.Fatalf("debit worker %d error: %v", i, debitErrs[i])
		}
		if debitIDs[i] == "" || debitIDs[i] != debitIDs[0] {
			t.Fatalf("debit worker %d txID=%q != %q", i, debitIDs[i], debitIDs[0])
		}
	}

	bal, _ = led.Balance(ctx, "idem-org", "", "usd", false)
	if bal != 200 {
		t.Fatalf("balance after duplicate concurrent debits=%d, want 200", bal)
	}

	refundIDs := make([]string, workers)
	refundErrs := make([]error, workers)
	runParallel(workers, func(i int) {
		id, err := led.Refund(ctx, RefundInput{
			Org: "idem-org",
			Ref: "debit-once",
		})
		refundIDs[i] = id
		refundErrs[i] = err
	})

	for i := 0; i < workers; i++ {
		if refundErrs[i] != nil {
			t.Fatalf("refund worker %d error: %v", i, refundErrs[i])
		}
		if refundIDs[i] == "" || refundIDs[i] != refundIDs[0] {
			t.Fatalf("refund worker %d txID=%q != %q", i, refundIDs[i], refundIDs[0])
		}
	}

	bal, _ = led.Balance(ctx, "idem-org", "", "usd", false)
	if bal != 500 {
		t.Fatalf("balance after duplicate concurrent refunds=%d, want 500", bal)
	}
	if sum := led.LedgerBalanceSum(); sum != 0 {
		t.Fatalf("double-entry sum=%d, want 0", sum)
	}
}

func TestCreditLedger_ConcurrentCreditsDebitsRefundsClawbacksAndReads(t *testing.T) {
	ctx := context.Background()
	led := NewMemCreditLedger()
	Set(led)
	defer Set(nil)

	if got := Get(); got != led {
		t.Fatalf("Get() returned unexpected ledger")
	}

	// Fund live + sandbox books with cash payments.
	for i := 0; i < 10; i++ {
		if _, _, err := led.Credit(ctx, CreditInput{
			Org:            "mixorg",
			Subject:        "pool",
			Currency:       "usd",
			IdempotencyKey: fmt.Sprintf("pay-%d", i),
			AmountCents:    200,
			Cash:           true,
		}); err != nil {
			t.Fatalf("seed live credit %d: %v", i, err)
		}
	}
	// Sandbox money must stay isolated from live money even with identical keys.
	if _, _, err := led.Credit(ctx, CreditInput{
		Org:            "mixorg",
		Subject:        "pool",
		Currency:       "usd",
		IdempotencyKey: "pay-0",
		AmountCents:    777,
		Test:           true,
		Cash:           true,
	}); err != nil {
		t.Fatalf("seed test credit: %v", err)
	}

	// Pre-record 5 debits we will concurrently refund alongside fresh credits & debits.
	for i := 0; i < 5; i++ {
		if _, _, err := led.Debit(ctx, DebitInput{
			Org:         "mixorg",
			Subject:     "pool",
			Currency:    "usd",
			Ref:         fmt.Sprintf("pre-debit-%d", i),
			AmountCents: 100,
		}); err != nil {
			t.Fatalf("pre-debit %d: %v", i, err)
		}
	}
	// Starting live balance: 2000 - 500 = 1500.

	var mu sync.Mutex
	var expectedLiveDelta int64

	const opsPerType = 15
	runParallel(opsPerType*4, func(i int) {
		kind := i % 4
		idx := i / 4
		switch kind {
		case 0:
			// Parallel cash credits (+15 * 100 = +1500)
			if _, _, err := led.Credit(ctx, CreditInput{
				Org:            "mixorg",
				Subject:        "pool",
				Currency:       "usd",
				IdempotencyKey: fmt.Sprintf("conc-credit-%d", idx),
				AmountCents:    100,
				Cash:           true,
			}); err != nil {
				t.Errorf("concurrent Credit %d: %v", idx, err)
				return
			}
			mu.Lock()
			expectedLiveDelta += 100
			mu.Unlock()
		case 1:
			// Parallel debits (may succeed or return ErrShort)
			_, _, err := led.Debit(ctx, DebitInput{
				Org:         "mixorg",
				Subject:     "pool",
				Currency:    "usd",
				Ref:         fmt.Sprintf("conc-debit-%d", idx),
				AmountCents: 120,
			})
			if err == nil {
				mu.Lock()
				expectedLiveDelta -= 120
				mu.Unlock()
			} else if !errors.Is(err, ErrShort) {
				t.Errorf("concurrent Debit %d: %v", idx, err)
			}
		case 2:
			// Refund pre-recorded debits (at most once each, indices 0..4) + concurrent Balance reads
			refIdx := idx % 5
			if _, err := led.Refund(ctx, RefundInput{
				Org:     "mixorg",
				Subject: "pool",
				Ref:     fmt.Sprintf("pre-debit-%d", refIdx),
			}); err != nil {
				t.Errorf("concurrent Refund %d: %v", refIdx, err)
			}
			if _, err := Get().Balance(ctx, "mixorg", "pool", "usd", false); err != nil {
				t.Errorf("concurrent Balance: %v", err)
			}
		case 3:
			// Parallel clawback & restore on pay-0 (idempotent for cb-0)
			if _, err := led.Clawback(ctx, ClawbackInput{
				Org:         "mixorg",
				Subject:     "pool",
				Payment:     "pay-0",
				Ref:         "cb-0",
				AmountCents: 50,
			}); err != nil {
				t.Errorf("concurrent Clawback: %v", err)
			}
		}
	})

	// All 5 pre-debits (5 * 100 = 500) were refunded once, and cb-0 (-50) was applied once.
	expectedAfterClawback := 1500 + expectedLiveDelta + 500 - 50
	liveBal, err := led.Balance(ctx, "mixorg", "pool", "usd", false)
	if err != nil {
		t.Fatalf("live Balance: %v", err)
	}
	if liveBal != expectedAfterClawback {
		t.Fatalf("live balance=%d, want %d", liveBal, expectedAfterClawback)
	}

	// Parallel idempotent Restore of cb-0 restores 50 cents exactly once.
	runParallel(20, func(_ int) {
		if _, err := led.Restore(ctx, ClawbackInput{
			Org:     "mixorg",
			Subject: "pool",
			Payment: "pay-0",
			Ref:     "cb-0",
		}); err != nil {
			t.Errorf("Restore: %v", err)
		}
	})

	liveBal, _ = led.Balance(ctx, "mixorg", "pool", "usd", false)
	if liveBal != expectedAfterClawback+50 {
		t.Fatalf("live balance after Restore=%d, want %d", liveBal, expectedAfterClawback+50)
	}

	testBal, _ := led.Balance(ctx, "mixorg", "pool", "usd", true)
	if testBal != 777 {
		t.Fatalf("sandbox balance=%d, want 777 (live traffic leaked into test book)", testBal)
	}
	if sum := led.LedgerBalanceSum(); sum != 0 {
		t.Fatalf("double-entry sum=%d, want 0", sum)
	}
}
