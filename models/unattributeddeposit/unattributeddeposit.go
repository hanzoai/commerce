// Package unattributeddeposit records money that arrived at a custody address
// we own and named NOBODY.
//
// It exists because of the POOLED deposit model. On chains where each payer is
// minted their own address, the address IS the answer to "whose money is this?"
// and this record is unreachable. On XRPL it is not: a non-refundable account
// reserve makes a fresh address per payer wasteful, so every payer shares one
// account and a DESTINATION TAG says whose deposit it is. A payment that
// carries no tag, or a tag we never issued, therefore reaches a destination we
// control while naming no one.
//
// Neither obvious thing to do with such a payment is acceptable. Crediting it
// means guessing which customer's balance to increase with someone else's
// money. Dropping it means a customer's real deposit disappears without trace
// once the scan window moves past it — the exact failure billing/depositwatch
// exists to end. So it is recorded here instead: durable, idempotent on the
// on-chain event, and credited to no one, leaving something an operator can
// refund from or attribute by hand.
//
// It carries NO money and is not part of any balance. It lives in the global
// "system" namespace beside the watcher's cursor, because it belongs to no org
// — not knowing which org it belongs to is the whole reason it is here.
package unattributeddeposit

import (
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/orm"
)

func init() {
	orm.Register[UnattributedDeposit]("unattributed-deposit", orm.WithStringKey[UnattributedDeposit]())
}

// UnattributedDeposit is one on-chain payment into a custody address that named
// no intent.
type UnattributedDeposit struct {
	mixin.Model[UnattributedDeposit]

	Chain string `json:"chain"`
	Token string `json:"token"`
	// Address is the pooled custody address the payment landed in.
	Address string `json:"address"`
	// Tag is the routing tag the payment carried. EMPTY means it carried none
	// at all, which is a different thing from the tag "0" — 0 is a legal tag
	// that some payer may genuinely hold.
	Tag string `json:"tag,omitempty"`
	// Units is the raw base-unit amount, decimal. It is a string for the same
	// reason a credit's is: token amounts do not fit an int64 and must not be
	// rounded through a float on their way into an audit record.
	Units       string    `json:"units"`
	TxHash      string    `json:"txHash"`
	EventIndex  uint64    `json:"eventIndex"`
	BlockNumber uint64    `json:"blockNumber"`
	FirstSeenAt time.Time `json:"firstSeenAt"`
}

func (u *UnattributedDeposit) Load(ps []datastore.Property) error  { return datastore.LoadStruct(u, ps) }
func (u *UnattributedDeposit) Save() ([]datastore.Property, error) { return datastore.SaveStruct(u) }

// New returns an initialized UnattributedDeposit bound to db.
func New(db *datastore.Datastore) *UnattributedDeposit {
	u := new(UnattributedDeposit)
	u.Init(db)
	return u
}

// Query lists unattributed deposits.
func Query(db *datastore.Datastore) datastore.Query { return db.Query("unattributed-deposit") }
