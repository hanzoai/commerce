// Package transfer records that a payable was PAID — the annotation a human
// writes after paying out-of-band. Commerce executes no payout; a Transfer is a
// fact about one that already happened.
package transfer

import (
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/orm"
)

func init() { orm.Register[Transfer]("transfer") }

// Type is HOW it was paid. The three are one shape — method, reference, money,
// when, who — so nothing branches on which one it is. Anything a method needs
// that the others do not goes in Reference.
type Type string

const (
	ETH   Type = "eth"   // on-chain; Reference is the tx hash
	Wire  Type = "wire"  // bank wire; Reference is the wire/confirmation ref
	Other Type = "other" // anything else; Reference describes it
)

// Transfer is one payment against one payable.
type Transfer struct {
	mixin.Model[Transfer]

	// FeeId is the payable this settles; PayeeId is who was paid.
	FeeId   string `json:"feeId"`
	PayeeId string `json:"payeeId,omitempty"`

	// Settles is how much of the payable this clears, in the payable's asset.
	// Sent is what actually left, in whatever asset it left in — the two differ
	// when a dollar-denominated payable is paid in ETH.
	Settles currency.Money `json:"settles"`
	Sent    currency.Money `json:"sent,omitempty"`

	// (Type, Reference) names one real-world payment, so recording it twice
	// settles once.
	Type      Type      `json:"type"`
	Reference string    `json:"reference,omitempty"`
	PaidAt    time.Time `json:"paidAt,omitempty"`
	Actor     string    `json:"actor,omitempty"`
	Note      string    `json:"note,omitempty"`
}

func New(db *datastore.Datastore) *Transfer {
	t := new(Transfer)
	t.Init(db)
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("transfer")
}
