// Package claimitem is a single claimed line of an order claim: it references
// one order line item (by ItemId — the order LineItem.Id(), i.e. its variant or
// product id), the Quantity being claimed, and the Reason it is claimed.
//
// A claim's items are their own rows (parent Claim by ClaimId), mirroring the
// gift-card / redemption relationship: the Claim is the request, the items are
// the append-only facts of what is being claimed. The refund/replacement amount
// on accept is a projection over these rows against the order's line prices —
// never a mutable counter — so re-reading is always the source of truth.
package claimitem

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/orm"
)

func init() { orm.Register[ClaimItem]("claim-item") }

// Claim reasons — why a line is being claimed.
const (
	ReasonDamaged   = "damaged"
	ReasonWrongItem = "wrong_item"
	ReasonMissing   = "missing"
	ReasonOther     = "other"
)

// ValidReason reports whether r is one of the recognized claim reasons.
func ValidReason(r string) bool {
	switch r {
	case ReasonDamaged, ReasonWrongItem, ReasonMissing, ReasonOther:
		return true
	default:
		return false
	}
}

// ClaimItem is one claimed order line. ItemId matches the order LineItem.Id()
// (variant id, else product id). Quantity is the number of units claimed and
// must be ≤ the quantity originally ordered on that line (enforced at accept).
type ClaimItem struct {
	mixin.Model[ClaimItem]

	// ClaimId is the parent claim this item belongs to.
	ClaimId string `json:"claimId"`

	// ItemId references the order line by its LineItem.Id() (variant or product).
	ItemId string `json:"itemId"`

	// Quantity is the number of units being claimed on this line.
	Quantity int `json:"quantity"`

	// Reason is one of damaged|wrong_item|missing|other.
	Reason string `json:"reason"`
}

func (i *ClaimItem) Load(ps []datastore.Property) error {
	return datastore.LoadStruct(i, ps)
}

func (i *ClaimItem) Save() ([]datastore.Property, error) {
	return datastore.SaveStruct(i)
}

func New(db *datastore.Datastore) *ClaimItem {
	i := new(ClaimItem)
	i.Init(db)
	return i
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("claim-item")
}
