package engine

import (
	"errors"
	"fmt"
	"sync"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/idempotencykey"
)

// ErrInvoiceBusy means another path holds the invoice's lock: a payment, a void
// or a tax change on it is under way.
var ErrInvoiceBusy = errors.New("a payment or change on this invoice is in progress")

// invoiceLockScope names the invoice leases among the idempotency records.
const invoiceLockScope = "invoice-lock"

// held is the invoices this process has locked, by namespace and id.
var held = struct {
	sync.Mutex
	ids map[string]bool
}{ids: map[string]bool{}}

// LockInvoice takes the one lock every path that moves money on an invoice, or
// changes what it owes, holds: the billing cycle, the customer's own payment,
// void and tax. It never waits: a held lock answers ErrInvoiceBusy, and the
// caller answers "in progress" or leaves the invoice for the next run.
//
// The lock is two claims. An in-process one serializes the paths inside the
// single writer commerce runs as. A lease row keyed by the invoice id is what
// another process honors; a holder that crashes leaves it to expire after
// idempotencykey.StartedTTL. A lease that cannot be read or written refuses the
// lock rather than guessing.
func LockInvoice(db *datastore.Datastore, invoiceID string) (release func(), err error) {
	key := db.GetNamespace() + "/" + invoiceID
	held.Lock()
	if held.ids[key] {
		held.Unlock()
		return nil, ErrInvoiceBusy
	}
	held.ids[key] = true
	held.Unlock()
	unhold := func() {
		held.Lock()
		delete(held.ids, key)
		held.Unlock()
	}

	lease, replay, err := idempotencykey.Begin(db, invoiceLockScope, invoiceID)
	if err != nil {
		unhold()
		return nil, fmt.Errorf("take the lease on invoice %s: %w", invoiceID, err)
	}
	if replay {
		unhold()
		return nil, ErrInvoiceBusy
	}
	return func() {
		_ = lease.Delete()
		unhold()
	}, nil
}
