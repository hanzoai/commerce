package engine

import (
	"errors"
	"testing"

	"github.com/hanzoai/commerce/models/idempotencykey"
)

// TestLockInvoice_OneHolderAtATime: a held invoice answers ErrInvoiceBusy
// without waiting, another invoice is unaffected, and a released lock can be
// taken again.
func TestLockInvoice_OneHolderAtATime(t *testing.T) {
	db, done := settleDB(t, "lock-one")
	defer done()

	release, err := LockInvoice(db, "inv_a")
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	if _, err := LockInvoice(db, "inv_a"); !errors.Is(err, ErrInvoiceBusy) {
		t.Fatalf("second lock on a held invoice: %v, want ErrInvoiceBusy", err)
	}
	other, err := LockInvoice(db, "inv_b")
	if err != nil {
		t.Fatalf("lock on another invoice: %v", err)
	}
	other()
	release()
	again, err := LockInvoice(db, "inv_a")
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	again()
}

// TestLockInvoice_HonorsAnotherProcessLease: a lease row another process holds
// refuses the lock until it expires.
func TestLockInvoice_HonorsAnotherProcessLease(t *testing.T) {
	db, done := settleDB(t, "lock-lease")
	defer done()
	if _, _, err := idempotencykey.Begin(db, invoiceLockScope, "inv_c"); err != nil {
		t.Fatalf("lease: %v", err)
	}
	if _, err := LockInvoice(db, "inv_c"); !errors.Is(err, ErrInvoiceBusy) {
		t.Fatalf("lock under another process's live lease: %v, want ErrInvoiceBusy", err)
	}
}

// TestLockInvoice_LeaseHoldsAfterARelease: the lease row has a deterministic id,
// so it is written, released and written again for every payment on an invoice.
// A lease one process takes after an earlier release still refuses the lock to
// another process that shares the store and not the first one's memory.
func TestLockInvoice_LeaseHoldsAfterARelease(t *testing.T) {
	db, done := settleDB(t, "lock-relock")
	defer done()
	const id = "inv_relock"

	first, err := LockInvoice(db, id)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	first()

	if _, replay, err := idempotencykey.Begin(db, invoiceLockScope, id); err != nil || replay {
		t.Fatalf("the other process's lease: replay=%v err=%v", replay, err)
	}
	if release, err := LockInvoice(db, id); !errors.Is(err, ErrInvoiceBusy) {
		if release != nil {
			release()
		}
		t.Fatalf("took an invoice another process holds a lease on (err=%v)", err)
	}
}
