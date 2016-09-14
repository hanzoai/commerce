package organization

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/pricing"
)

func (o Organization) Kind() string {
	return "organization"
}

func (o *Organization) Init(db *datastore.Datastore) {
	o.Model.Init(db, o)
	o.AccessToken.Init(o)
}

func (o *Organization) Defaults() {
	o.Admins = make([]string, 0)
	o.Moderators = make([]string, 0)

	o.Fees.Id = o.Id()
	o.Fees.Card.Flat = 50
	o.Fees.Card.Percent = 0.05
	o.Fees.Affiliate.Flat = 30
	o.Fees.Affiliate.Percent = 0.30

	o.Partners = make([]pricing.Partner, 0)
}

func New(db *datastore.Datastore) *Organization {
	o := new(Organization)
	o.Init(db)
	o.Defaults()
	return o
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
