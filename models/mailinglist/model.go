package mailinglist

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/types/thankyou"
)

var kind = "mailinglist"

func (m MailingList) Kind() string {
	return kind
}

func (m *MailingList) Init(db *datastore.Datastore) {
	m.Model.Init(db, m)
}

func (m *MailingList) Defaults() {
	m.Facebook.Value = "0.00"
	m.Facebook.Currency = "USD"
	m.ThankYou.Type = thankyou.Disabled
	m.Mailchimp.Enabled = true
	m.Type = "subscribe"
}

func New(db *datastore.Datastore) *MailingList {
	m := new(MailingList)
	m.Init(db)
	return m
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
