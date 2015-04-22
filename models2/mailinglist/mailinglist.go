package mailinglist

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/subscriber"
	"crowdstart.io/util/val"
)

type Mailchimp struct {
	Id               string `json:"id"`
	APIKey           string `json:"apiKey"`
	DoubleOptin      bool   `json:"doubleOptin"`
	UpdateExisting   bool   `json:"updateExisting"`
	ReplaceInterests bool   `json:"replaceInterests"`
}

type MailingList struct {
	mixin.Model

	Name      string    `json:"name"`
	Mailchimp Mailchimp `json:"mailchimp"`
}

func New(db *datastore.Datastore) *MailingList {
	m := new(MailingList)
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
	function findForm() {
		// start at the root element
		var node = document.documentElement;
		while (node.childNodes.length && node.lastChild.nodeType == 1) {
			// find last HTMLElement child node
			node = node.lastChild;
		}
		// node is now the script element
		form = node.parentNode;

		return form
	}

	function attachForm() {
		var form = findForm()
	}
`
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
