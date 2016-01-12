package referral

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

type Referral struct {
	mixin.Model

	// User being referred
	UserId string `json:"userId"`

	// Associated order
	OrderId string `json:"orderId"`

	// Referred by
	ReferrerUserId string `json:"referrerUserId"`
	ReferrerId     string `json:"referrerId"`
}

func (r Referral) Kind() string {
	return "referral"
}
func (r *Referral) Init(db *datastore.Datastore) {
	r.Model = mixin.Model{Db: db, Entity: r}
}

func New(db *datastore.Datastore) *Referral {
	return new(Referral).New(db).(*Referral)
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
