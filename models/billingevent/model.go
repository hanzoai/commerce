package billingevent

import "github.com/hanzoai/commerce/datastore"

var kind = "billing-event"

func (e BillingEvent) Kind() string {
	return kind
}

func (e *BillingEvent) Init(db *datastore.Datastore) {
	e.Model.Init(db, e)
}

func (e *BillingEvent) Defaults() {
	e.Parent = e.Db.NewKey("synckey", "", 1, nil)
	e.Pending = true
	e.Livemode = true
}

func New(db *datastore.Datastore) *BillingEvent {
	e := new(BillingEvent)
	e.Init(db)
	e.Defaults()
	return e
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
