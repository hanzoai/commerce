// Package draftorder is the admin order-builder domain (Medusa v2 parity:
// admin/draft-orders). A draft order is an order an admin composes on a
// customer's behalf — a currency, a customer reference, and a set of line
// items — that is later converted into a REAL order (models/order) with the
// same items and total.
//
// Line items are separate facts (models/draftorderitem), one row per line, so
// the builder can add/remove them independently. The draft's total is a
// PROJECTION over those lines (Σ unitPrice × qty), never a mutable counter —
// the same values-not-places design the gift-card ledger uses.
package draftorder

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/orm"

	. "github.com/hanzoai/commerce/types"
)

func init() { orm.Register[DraftOrder]("draft-order") }

// Draft order lifecycle states.
const (
	StatusDraft    = "draft"
	StatusComplete = "complete"
)

// DraftOrder is an in-progress order an admin builds for a customer. It holds
// no line items itself; those are draftorderitem rows keyed by DraftOrderId.
// Once completed, OrderId points at the real order it produced and Status is
// StatusComplete (terminal).
type DraftOrder struct {
	mixin.Model[DraftOrder]

	// Customer this draft is being built for (optional until complete).
	CustomerId string `json:"customerId,omitempty"`
	Email      string `json:"email,omitempty"`

	Currency currency.Type `json:"currency" orm:"default:usd"`

	Status string `json:"status" orm:"default:draft"`

	// OrderId is the real order this draft was converted into (set on complete).
	OrderId string `json:"orderId,omitempty"`

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (d *DraftOrder) Load(ps []datastore.Property) (err error) {
	if err = datastore.LoadStruct(d, ps); err != nil {
		return err
	}
	if len(d.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(d.Metadata_), &d.Metadata)
	}
	return err
}

func (d *DraftOrder) Save() ([]datastore.Property, error) {
	d.Metadata_ = string(json.EncodeBytes(&d.Metadata))
	return datastore.SaveStruct(d)
}

// IsDraft reports whether the draft can still be edited and completed. An empty
// status is treated as draft (the orm default) so a freshly-created row is
// editable before its first save round-trips the default.
func (d *DraftOrder) IsDraft() bool {
	return d.Status == "" || d.Status == StatusDraft
}

func New(db *datastore.Datastore) *DraftOrder {
	d := new(DraftOrder)
	d.Init(db)
	return d
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("draft-order")
}
