// Package claim is the order-claim domain (Medusa v2 core parity: order
// claims): a customer reports a problem with delivered items (damaged, wrong,
// missing) and the merchant resolves it with either a refund or a replacement
// order.
//
// A Claim references an order and carries its claimed lines as claimitem rows
// (parent by ClaimId). It is pending until the merchant accepts (→ refund or
// replacement) or rejects it. Accept is idempotent: an already-accepted claim
// returns its prior outcome (RefundId / ReplacementOrderId) without moving money
// or creating a second refund. AmountCents is a projection computed at accept
// time from the claimed quantities × the order's line prices — never a mutable
// counter — so it is exact and re-derivable.
package claim

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/orm"

	. "github.com/hanzoai/commerce/types"
)

func init() { orm.Register[Claim]("claim") }

// Claim lifecycle statuses.
const (
	StatusPending  = "pending"
	StatusAccepted = "accepted"
	StatusRejected = "rejected"
)

// Resolution types — how an accepted claim is settled.
const (
	ResolutionRefund  = "refund"
	ResolutionReplace = "replace"
)

// ValidResolution reports whether r is a recognized resolution type.
func ValidResolution(r string) bool {
	return r == ResolutionRefund || r == ResolutionReplace
}

// Claim links a problem report to an order and resolves to a refund or a
// replacement order. Its claimed lines live as claimitem rows keyed by ClaimId.
type Claim struct {
	mixin.Model[Claim]

	// OrderId is the order this claim is filed against.
	OrderId string `json:"orderId"`

	// Resolution is refund|replace. Default refund.
	Resolution string `json:"resolution" orm:"default:refund"`

	// Status is pending|accepted|rejected. Default pending.
	Status string `json:"status" orm:"default:pending"`

	// Reason is an optional free-text summary of the claim (per-line reasons
	// live on the claim items).
	Reason string `json:"reason,omitempty"`

	CurrencyCode currency.Type `json:"currencyCode" orm:"default:usd"`

	// AmountCents is the settled amount, computed at accept from the claimed
	// quantities × the order line prices. Zero until accepted.
	AmountCents currency.Cents `json:"amountCents"`

	// RefundId is set when an accepted claim was resolved with a refund.
	RefundId string `json:"refundId,omitempty"`

	// ReplacementOrderId is set when an accepted claim was resolved with a
	// replacement order.
	ReplacementOrderId string `json:"replacementOrderId,omitempty"`

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (cl *Claim) Load(ps []datastore.Property) (err error) {
	if err = datastore.LoadStruct(cl, ps); err != nil {
		return err
	}
	if len(cl.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(cl.Metadata_), &cl.Metadata)
	}
	return err
}

func (cl *Claim) Save() ([]datastore.Property, error) {
	cl.Metadata_ = string(json.EncodeBytes(&cl.Metadata))
	return datastore.SaveStruct(cl)
}

// IsOpen reports whether the claim is still awaiting a decision.
func (cl *Claim) IsOpen() bool {
	return cl.Status == StatusPending
}

func New(db *datastore.Datastore) *Claim {
	cl := new(Claim)
	cl.Init(db)
	return cl
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("claim")
}
