package mailinglist

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/subscriber"
	"crowdstart.io/util/val"
)

type MailchimpList struct {
	Id               string `json:"id"`
	APIKey           string `json:"apiKey"`
	DoubleOptin      bool   `json:"doubleOptin"`
	UpdateExisting   bool   `json:"updateExisting"`
	ReplaceInterests bool   `json:"replaceInterests"`
}

type MailingList struct {
	mixin.Model

	Name          string        `json:"name"`
	MailchimpList MailchimpList `json:"mailchimp"`
}

func (m *MailingList) Init() {
}

func New(db *datastore.Datastore) *MailingList {
	m := new(MailingList)
	m.Init()
	m.Model = mixin.Model{Db: db, Entity: m}
	return m
}

func (m MailingList) Kind() string {
	return "mailinglist"
}

func (m *MailingList) Validator() *val.Validator {
	return val.New(m)
}

func (m *MailingList) AddSubscriber(s *subscriber.Subscriber) error {
	s.MailingListId = s.Id()
	s.Parent = m.Key()
	return s.Put()
}

func (m *MailingList) Js() string {
	return `
	`
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
