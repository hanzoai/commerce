package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/creditgrant"
	"github.com/hanzoai/commerce/types"
)

// ReturnPaid gives back what an invoice collected toward a period that will not
// be served, before it is voided or written off as uncollectible: the credit it
// applied (CreditApplied) as a credit grant, and the rest to the subject's
// balance on the one ledger (p.Return). Both are keyed by the invoice, so
// returning twice returns once, and the invoice records what was returned in
// Metadata["returnedCents"]. The caller persists the invoice.
//
// The rest goes to the balance whatever collected it: a card leg that settled
// before a prepaid leg failed is given back as prepaid money the customer can
// spend, not refunded to the card.
func ReturnPaid(ctx context.Context, db *datastore.Datastore, inv *billinginvoice.BillingInvoice, p Prepaid, now time.Time) error {
	if inv.AmountPaid <= 0 {
		return nil
	}
	if _, done := inv.Metadata["returnedCents"]; done {
		return nil
	}
	cur := invoiceCurrency(inv)
	credit := min(inv.CreditApplied, inv.AmountPaid)
	if credit > 0 {
		g := creditgrant.New(db)
		if err := g.SetKey(returnKey(db, inv)); err != nil {
			return err
		}
		switch err := g.Get(g.Key()); {
		case err == nil:
		case errors.Is(err, datastore.ErrNoSuchEntity):
			g.UserId = inv.UserId
			g.Name = "Returned from invoice " + inv.NumberStr
			g.AmountCents, g.RemainingCents = credit, credit
			g.Currency = cur
			g.EffectiveAt = now
			g.Tags = "invoice-return"
			g.Metadata = types.Map{"invoiceId": inv.Id()}
			if err := g.Create(); err != nil {
				return fmt.Errorf("return %d cents of credit from invoice %s: %w", credit, inv.Id(), err)
			}
		default:
			return fmt.Errorf("read the credit returned from invoice %s: %w", inv.Id(), err)
		}
	}
	if rest := inv.AmountPaid - credit; rest > 0 {
		if p == nil {
			return fmt.Errorf("invoice %s collected %d cents and there is no ledger to return them to", inv.Id(), rest)
		}
		if _, err := p.Return(ctx, inv.UserId, cur, rest, "invoice-return:"+inv.Id()); err != nil {
			return fmt.Errorf("return %d cents to the balance from invoice %s: %w", rest, inv.Id(), err)
		}
	}
	if inv.Metadata == nil {
		inv.Metadata = types.Map{}
	}
	inv.Metadata["returnedCents"] = inv.AmountPaid
	return nil
}

// returnKey is the deterministic storage key of the credit grant an invoice's
// return writes.
func returnKey(db *datastore.Datastore, inv *billinginvoice.BillingInvoice) datastore.Key {
	sum := sha256.Sum256([]byte("invoice-return\x00credit-grant\x00" + inv.Id()))
	return db.NewKey("credit-grant", "rtrn_"+hex.EncodeToString(sum[:16]), 0, db.NewKey("synckey", "", 1, nil))
}
