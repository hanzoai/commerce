package form

import (
	"hanzo.io/datastore"
	"hanzo.io/models/types/thankyou"
)

var kind = "form"

func (m Form) Kind() string {
	return kind
}

func (m *Form) Init(db *datastore.Datastore) {
	m.Model.Init(db, m)
}

func (m *Form) Defaults() {
	m.Facebook.Value = "0.00"
	m.Facebook.Currency = "USD"
	m.ThankYou.Type = thankyou.Disabled
	m.EmailList.Enabled = true
	m.Type = "subscribe"
}

func New(db *datastore.Datastore) *Form {
	m := new(Form)
	m.Init(db)
	m.Defaults()
	return m
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
