package partner

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/commission"
	"github.com/hanzoai/commerce/models/types/schedule"
	"github.com/hanzoai/orm"

	. "github.com/hanzoai/commerce/types"
)

func init() { orm.Register[Partner]("partner") }

type Partner struct {
	mixin.Model[Partner]

	Enabled   bool `json:"enabled"`
	Connected bool `json:"connected"`

	Name     string  `json:"name"`
	Email    string  `json:"email,omitempty"`
	Phone    string  `json:"phone,omitempty"`
	Address  Address `json:"address,omitempty"`
	Website  string  `json:"website,omitempty"`
	Country  string  `json:"country"`
	TaxId    string  `json:"taxId"`
	Timezone string  `json:"timezone"`

	Commission commission.Commission `json:"commission"`
	Schedule   schedule.Schedule     `json:"schedule"`

	// Wallet is how this partner is PAID: a web3 address they connect
	// themselves. Payouts settle in crypto (or by wire, arranged off this
	// record) — there is no processor account id here, because Hanzo does
	// not disburse through a payment processor. An empty wallet does not
	// stop a fee from being EARNED; it only means the transfer that records
	// what is owed has nowhere to send it yet.
	Wallet string `json:"wallet,omitempty"`
}

// New creates a new Partner wired to the given datastore.
func New(db *datastore.Datastore) *Partner {
	p := new(Partner)
	p.Init(db)
	return p
}

// Query returns a datastore query for partners.
func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("partner")
}
