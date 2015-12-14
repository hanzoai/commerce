package referral

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/util/val"
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

func New(db *datastore.Datastore) *Referral {
	r := new(Referral)
	r.Model = mixin.Model{Db: db, Entity: r}
	return r
}

func (r Referral) Init() {
}

func (r Referral) Kind() string {
	return "referral"
}

func (r *Referral) Validator() *val.Validator {
	return nil
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
