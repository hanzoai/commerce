package affiliate

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/commission"
	"github.com/hanzoai/commerce/models/types/schedule"
	"github.com/hanzoai/orm"
)

func init() { orm.Register[Affiliate]("affiliate") }

type Affiliate struct {
	mixin.Model[Affiliate]

	Enabled   bool `json:"enabled"`
	Connected bool `json:"connected"`

	UserId   string `json:"userId,omitempty"`
	Name     string `json:"name,omitempty"`
	Company  string `json:"company,omitempty"`
	Country  string `json:"country,omitempty"`
	TaxId    string `json:"taxId,omitempty"`
	Timezone string `json:"timezone,omitempty"`

	Commission commission.Commission `json:"commission,omitempty"`
	Schedule   schedule.Schedule     `json:"schedule,omitempty"`
	CouponId   string                `json:"couponId,omitempty"`

	// Wallet is how this affiliate is PAID: a web3 address they connect
	// themselves. Payouts settle in crypto (or by wire, arranged off this
	// record) — there is no processor account id here, because Hanzo does
	// not disburse through a payment processor. An empty wallet does not
	// stop a fee from being EARNED; it only means the transfer that records
	// what is owed has nowhere to send it yet.
	Wallet string `json:"wallet,omitempty"`
}

func New(db *datastore.Datastore) *Affiliate {
	a := new(Affiliate)
	a.Init(db)
	a.Schedule.Period = 30
	a.Schedule.Type = schedule.DailyRolling
	return a
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("affiliate")
}
