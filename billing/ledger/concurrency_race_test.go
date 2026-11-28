package ledger

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemLedger_ConcurrentDuplicateIdempotencyKeys(t *testing.T) {
	ctx, l := setup(t)
	cash, _, custBal := createTestAccounts(t, ctx, l, "t-race")

	const workers = 50
	errs := make([]error, workers)
	ids := make([]string, workers)

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			e := &Entry{
				TenantID:       "t-race",
				IdempotencyKey: "dup-key",
				Description:    "concurrent debit",
				Postings: []Posting{
					{AccountID: cash.ID, Amount: 500, Currency: "usd"},
					{AccountID: custBal.ID, Amount: -500, Currency: "usd"},
				},
			}
			err := l.PostEntry(ctx, e)
			errs[i] = err
			ids[i] = e.ID
			// Mutate returned slice to confirm internal storage was cloned.
			if len(e.Postings) > 0 {
				e.Postings[0].Amount = 999999
			}
			// Concurrent balance read must not race with internal map mutations.
			if _, berr := l.GetBalance(ctx, cash.ID, "usd"); berr != nil {
				t.Errorf("GetBalance: %v", berr)
			}
		}()
	}
	close(start)
	wg.Wait()

	var okCount, dupCount int
	for i := 0; i < workers; i++ {
		switch {
		case errs[i] == nil:
			okCount++
		case errors.Is(errs[i], ErrDuplicateEntry):
			dupCount++
		default:
			t.Fatalf("worker %d unexpected error: %v", i, errs[i])
		}
		if ids[i] == "" || ids[i] != ids[0] {
			t.Fatalf("worker %d returned ID %q != %q", i, ids[i], ids[0])
		}
	}
	if okCount != 1 || dupCount != workers-1 {
		t.Fatalf("expected 1 success and %d duplicates, got ok=%d dup=%d", workers-1, okCount, dupCount)
	}

	bal, err := l.GetBalance(ctx, cash.ID, "usd")
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if bal.PostedBalance != 500 {
		t.Fatalf("cash PostedBalance=%d, want 500", bal.PostedBalance)
	}
	stored, err := l.GetEntry(ctx, ids[0])
	if err != nil {
		t.Fatalf("GetEntry: %v", err)
	}
	if stored.Postings[0].Amount != 500 {
		t.Fatalf("caller slice mutation corrupted stored posting: got %d, want 500", stored.Postings[0].Amount)
	}
}

func TestMemLedger_ConcurrentPaymentsRefundsPayoutsAndBalanceReads(t *testing.T) {
	ctx, l := setup(t)
	cash, fees, custBal := createTestAccounts(t, ctx, l, "t-mix")
	merch, err := l.EnsureAccount(ctx, "t-mix", "merchant_settlement:m1", Liability, "usd")
	if err != nil {
		t.Fatalf("EnsureAccount merchant: %v", err)
	}
	disp, err := l.EnsureAccount(ctx, "t-mix", "platform:disputes_held", Asset, "usd")
	if err != nil {
		t.Fatalf("EnsureAccount disputes: %v", err)
	}

	const ops = 25
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(ops * 4)
	for i := 0; i < ops; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			e, rerr := l.RecordPayment(ctx, "t-mix", fmt.Sprintf("pi-%d", i), 1000, "usd", "cust_1", 50)
			if rerr != nil {
				t.Errorf("RecordPayment %d: %v", i, rerr)
				return
			}
			if _, gerr := l.GetEntry(ctx, e.ID); gerr != nil {
				t.Errorf("GetEntry %d: %v", i, gerr)
			}
		}()
		go func() {
			defer wg.Done()
			<-start
			if _, rerr := l.RecordRefund(ctx, "t-mix", fmt.Sprintf("rf-%d", i), 200, "usd", "cust_1"); rerr != nil {
				t.Errorf("RecordRefund %d: %v", i, rerr)
			}
			if _, lerr := l.ListEntries(ctx, EntryFilter{TenantID: "t-mix", Limit: 5}); lerr != nil {
				t.Errorf("ListEntries: %v", lerr)
			}
		}()
		go func() {
			defer wg.Done()
			<-start
			if _, perr := l.RecordPayout(ctx, "t-mix", fmt.Sprintf("po-%d", i), 300, "usd", "m1"); perr != nil {
				t.Errorf("RecordPayout %d: %v", i, perr)
			}
			if _, berr := l.GetBalance(ctx, cash.ID, "usd"); berr != nil {
				t.Errorf("GetBalance cash: %v", berr)
			}
		}()
		go func() {
			defer wg.Done()
			<-start
			if _, derr := l.RecordDispute(ctx, "t-mix", fmt.Sprintf("dp-%d", i), 100, "usd", "cust_1"); derr != nil {
				t.Errorf("RecordDispute %d: %v", i, derr)
			}
			if _, berr := l.GetBalance(ctx, custBal.ID, "usd"); berr != nil {
				t.Errorf("GetBalance cust: %v", berr)
			}
		}()
	}
	close(start)
	wg.Wait()

	var sum int64
	for _, acctID := range []string{cash.ID, fees.ID, custBal.ID, merch.ID, disp.ID} {
		b, berr := l.GetBalance(ctx, acctID, "usd")
		if berr != nil {
			t.Fatalf("GetBalance %s: %v", acctID, berr)
		}
		sum += b.PostedBalance
	}
	if sum != 0 {
		t.Fatalf("double-entry sum across all tenant accounts=%d, want 0", sum)
	}
}

func TestMemLedger_ConcurrentHoldsCapturesVoidsAndRollback(t *testing.T) {
	ctx, l := setup(t)
	cash, _, custBal := createTestAccounts(t, ctx, l, "t-hold")

	// Rollback invariant: CaptureHold without platform:cash must not mutate hold/balance.
	noCashAcct, err := l.EnsureAccount(ctx, "t-nocash", "customer_balance:c1", Liability, "usd")
	if err != nil {
		t.Fatalf("EnsureAccount noCash: %v", err)
	}
	noCashHold := &Hold{
		TenantID:  "t-nocash",
		AccountID: noCashAcct.ID,
		Amount:    500,
		Currency:  "usd",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := l.CreateHold(ctx, noCashHold); err != nil {
		t.Fatalf("CreateHold noCash: %v", err)
	}
	if _, err := l.CaptureHold(ctx, noCashHold.ID, 500); err == nil {
		t.Fatal("expected error capturing hold without platform:cash")
	}
	if noCashHold.Status != HoldPending {
		t.Fatalf("hold status mutated on failed CaptureHold: got %s, want %s", noCashHold.Status, HoldPending)
	}
	nb, _ := l.GetBalance(ctx, noCashAcct.ID, "usd")
	if nb.HeldBalance != 500 {
		t.Fatalf("HeldBalance mutated on failed CaptureHold: got %d, want 500", nb.HeldBalance)
	}

	// Race CaptureHold vs VoidHold on many concurrent holds.
	if err := l.PostEntry(ctx, &Entry{
		TenantID:       "t-hold",
		IdempotencyKey: "seed",
		Postings: []Posting{
			{AccountID: cash.ID, Amount: 100000, Currency: "usd"},
			{AccountID: custBal.ID, Amount: -100000, Currency: "usd"},
		},
	}); err != nil {
		t.Fatalf("seed PostEntry: %v", err)
	}

	const pairs = 20
	var capturedCount, voidedCount atomic.Int64
	for i := 0; i < pairs; i++ {
		h := &Hold{
			TenantID:        "t-hold",
			AccountID:       custBal.ID,
			Amount:          100,
			Currency:        "usd",
			PaymentIntentID: fmt.Sprintf("pi_h_%d", i),
			ExpiresAt:       time.Now().Add(time.Hour),
		}
		if err := l.CreateHold(ctx, h); err != nil {
			t.Fatalf("CreateHold %d: %v", i, err)
		}

		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(3)
		var capErr, voidErr error
		go func() {
			defer wg.Done()
			<-start
			_, capErr = l.CaptureHold(ctx, h.ID, 100)
		}()
		go func() {
			defer wg.Done()
			<-start
			voidErr = l.VoidHold(ctx, h.ID)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, _ = l.GetBalance(ctx, custBal.ID, "usd")
		}()
		close(start)
		wg.Wait()

		switch {
		case capErr == nil && errors.Is(voidErr, ErrHoldNotPending):
			capturedCount.Add(1)
		case voidErr == nil && errors.Is(capErr, ErrHoldNotPending):
			voidedCount.Add(1)
		default:
			t.Fatalf("pair %d invalid capture/void outcome: capErr=%v voidErr=%v", i, capErr, voidErr)
		}
	}

	if capturedCount.Load()+voidedCount.Load() != pairs {
		t.Fatalf("expected %d terminal hold outcomes, got %d captured + %d voided", pairs, capturedCount.Load(), voidedCount.Load())
	}
	bal, err := l.GetBalance(ctx, custBal.ID, "usd")
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if bal.HeldBalance != 0 {
		t.Fatalf("HeldBalance=%d after all holds resolved, want 0", bal.HeldBalance)
	}
}
