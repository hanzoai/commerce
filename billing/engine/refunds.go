package engine

import (
	"fmt"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/credit"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/paymentorg"
)

// A payment's refunds and dispute — on its invoice, or on a wallet payment's record
// — are recorded only from the processor's signed webhook, through here, and each
// is a read-modify-write that swaps its row in (mixin.Model.Swap): a report another
// writer beat to the row reads it again, on this replica or another, so two reports
// both land and a report delivered twice is one entry. Commerce never makes a refund.

// RecordRefund records on invoice id the refund the processor gave back under its
// own refundID, once — a refund reported twice is one entry.
func RecordRefund(db *datastore.Datastore, id, refundID string, amount int64) error {
	return recordOn(db, id, func(inv *billinginvoice.BillingInvoice) bool {
		return inv.Refund("refund:"+refundID, amount)
	})
}

// RecordDispute records on invoice id the state a dispute against its payment
// reached, ordered by the dispute's version (or updated_at): a report older than
// the one the invoice holds changes nothing.
func RecordDispute(db *datastore.Datastore, id, state string, version int64, at time.Time) error {
	return recordOn(db, id, func(inv *billinginvoice.BillingInvoice) bool {
		return inv.SetDispute(state, version, at)
	})
}

// recordOn reads invoice id, applies change, and swaps the invoice in, reading again
// when another writer changed it first. A change that moves nothing writes nothing.
func recordOn(db *datastore.Datastore, id string, change func(*billinginvoice.BillingInvoice) bool) error {
	for range mixin.Swaps {
		inv := billinginvoice.New(db)
		if err := inv.GetById(id); err != nil {
			return err
		}
		if !change(inv) {
			return nil
		}
		ok, err := inv.Swap()
		if err != nil || ok {
			return err
		}
	}
	return fmt.Errorf("invoice %s changed under %d reads in a row", id, mixin.Swaps)
}

// RecordPaymentDispute records the state a dispute of a wallet payment reached on
// the payment's record (models/paymentorg), and reports whether it is the latest
// known — the one report whose money the caller may move.
func RecordPaymentDispute(payment, state string, version int64, at time.Time) (bool, error) {
	return paymentorg.RecordDispute(payment, state, version, at)
}

// CreateCreditNoteParams holds the parameters for creating a credit note.
type CreateCreditNoteParams struct {
	InvoiceId       string
	CustomerId      string
	Amount          int64
	Reason          string
	LineItems       []credit.CreditNoteLineItem
	OutOfBandAmount int64
	Memo            string
}

// CreateCreditNote creates a credit note against an invoice.
func CreateCreditNote(db *datastore.Datastore, params CreateCreditNoteParams) (*credit.CreditNote, error) {
	if params.InvoiceId == "" {
		return nil, fmt.Errorf("invoiceId is required")
	}

	inv := billinginvoice.New(db)
	if err := inv.GetById(params.InvoiceId); err != nil {
		return nil, fmt.Errorf("invoice not found: %w", err)
	}

	cn := credit.New(db)
	cn.InvoiceId = params.InvoiceId
	cn.CustomerId = params.CustomerId
	if cn.CustomerId == "" {
		cn.CustomerId = inv.UserId
	}
	cn.Currency = inv.Currency
	cn.Reason = params.Reason
	cn.Memo = params.Memo
	cn.LineItems = params.LineItems
	cn.OutOfBandAmount = params.OutOfBandAmount

	// Calculate total amount from line items
	if params.Amount > 0 {
		cn.Amount = params.Amount
	} else {
		var total int64
		for _, li := range params.LineItems {
			total += li.Amount
		}
		total += params.OutOfBandAmount
		cn.Amount = total
	}

	// Auto-number (simple increment based on query count)
	rootKey := db.NewKey("synckey", "", 1, nil)
	existing := make([]*credit.CreditNote, 0)
	if _, err := credit.Query(db).Ancestor(rootKey).GetAll(&existing); err == nil {
		cn.SetNumber(len(existing) + 1)
	} else {
		cn.SetNumber(1)
	}

	if err := cn.Create(); err != nil {
		return nil, fmt.Errorf("failed to create credit note: %w", err)
	}

	return cn, nil
}
