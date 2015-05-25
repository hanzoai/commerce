package referralinstance

import (
	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/referral"
	"crowdstart.com/util/val"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type ReferralInstance struct {
	mixin.Model

	Referral *referral.Referral `json:"referral"`
	OrderId  string             `json:"orderId"`
	UserId   string             `json:"userId"`
}

func New(db *datastore.Datastore) *ReferralInstance {
	r := new(ReferralInstance)
	r.Model = mixin.Model{Db: db, Entity: r}
	return r
}

func (r ReferralInstance) Kind() string {
	return "referralinstance"
}

func (r *ReferralInstance) Load(c <-chan aeds.Property) (err error) {
	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(r, c)); err != nil {
		return err
	}

	return err
}

func (r *ReferralInstance) Save(c chan<- aeds.Property) (err error) {
	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(r, c))
}

func (r *ReferralInstance) Validator() *val.Validator {
	return nil
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
