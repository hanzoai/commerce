// Package draftorderitem is one line of a draft order (Medusa v2 parity:
// admin/draft-orders items). Each item is an immutable-shaped fact — a
// product/variant reference, a quantity, and a server-authoritative unit price
// — keyed to its parent by DraftOrderId. The draft's total is the projection
// Σ(unitPrice × qty) over these rows (see draftorder.TotalCents), so adding or
// removing a line never races a mutable order-total counter.
package draftorderitem

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/orm"
)

func init() { orm.Register[DraftOrderItem]("draft-order-item") }

// DraftOrderItem is a single line of a draft order. It references either a
// variant (VariantId) or a bare product (ProductId); the cached name is stored
// for display. UnitPriceCents is the price the admin set for this line — it is
// server-authoritative and carried verbatim onto the real order line when the
// draft is completed.
type DraftOrderItem struct {
	mixin.Model[DraftOrderItem]

	// DraftOrderId is the parent draft this line belongs to.
	DraftOrderId string `json:"draftOrderId"`

	ProductId   string `json:"productId,omitempty"`
	ProductName string `json:"productName,omitempty"`
	VariantId   string `json:"variantId,omitempty"`
	VariantName string `json:"variantName,omitempty"`

	Quantity       int            `json:"quantity"`
	UnitPriceCents currency.Cents `json:"unitPriceCents"`
	Currency       currency.Type  `json:"currency" orm:"default:usd"`
}

// TotalCents is the extended price of this line: unit price × quantity.
func (i *DraftOrderItem) TotalCents() currency.Cents {
	return i.UnitPriceCents * currency.Cents(i.Quantity)
}

func (i *DraftOrderItem) Load(ps []datastore.Property) error {
	return datastore.LoadStruct(i, ps)
}

func (i *DraftOrderItem) Save() ([]datastore.Property, error) {
	return datastore.SaveStruct(i)
}

func New(db *datastore.Datastore) *DraftOrderItem {
	i := new(DraftOrderItem)
	i.Init(db)
	return i
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("draft-order-item")
}
