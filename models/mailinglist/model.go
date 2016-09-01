package mailinglist

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/thankyou"
)

func (m MailingList) Kind() string {
	return "mailinglist"
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

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
