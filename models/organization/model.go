package organization

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/types/pricing"
)

var kind = "organization"

func (o Organization) Kind() string {
	return kind
}

func (o *Organization) Init(db *datastore.Datastore) {
	o.Model.Init(db, o)
	o.AccessToken.Init(o)
}

func (o *Organization) Defaults() {
	o.Admins = make([]string, 0)
	o.Moderators = make([]string, 0)

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

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
