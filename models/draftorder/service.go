package draftorder

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/draftorderitem"
	"github.com/hanzoai/commerce/models/types/currency"
)

// Items returns the line items of a draft order, scoped to db's namespace.
// The lines are facts; the draft's total is projected from them (TotalCents).
func Items(db *datastore.Datastore, draftOrderId string) ([]*draftorderitem.DraftOrderItem, error) {
	items := make([]*draftorderitem.DraftOrderItem, 0, 16)
	if _, err := draftorderitem.Query(db).
		Filter("DraftOrderId=", draftOrderId).
		GetAll(&items); err != nil {
		return nil, err
	}
	return items, nil
}

// TotalCents projects the draft order total from its line items:
// Σ(unitPrice × quantity). It is derived, never stored, so add/remove of a line
// can never leave a stale total behind.
func TotalCents(items []*draftorderitem.DraftOrderItem) currency.Cents {
	var total currency.Cents
	for _, i := range items {
		total += i.TotalCents()
	}
	return total
}
