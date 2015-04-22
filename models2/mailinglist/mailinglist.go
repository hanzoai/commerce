package mailinglist

import (
	"fmt"
	"os"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/subscriber"
	"crowdstart.io/util/fs"
	"crowdstart.io/util/val"
)

var jsTemplate = ""

type Mailchimp struct {
	Id               string `json:"id"`
	APIKey           string `json:"apiKey"`
	DoubleOptin      bool   `json:"doubleOptin"`
	UpdateExisting   bool   `json:"updateExisting"`
	ReplaceInterests bool   `json:"replaceInterests"`
}

type MailingList struct {
	mixin.Model

	// Name of list
	Name string `json:"name"`

	// Mailchimp settings for this list
	Mailchimp Mailchimp `json:"mailchimp"`

	// Url to Thank you page
	ThankYou string `json:"thankyou,omitempty"`

	// Conversion tracking info
	Facebook struct {
		Id       string `json:"id"`
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"facebook"`

	Google struct {
		Category string `json:"category"`
		Name     string `json:"name"`
	} `json:"google"`
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
	if jsTemplate == "" {
		var cwd, _ = os.Getwd()
		jsTemplate = string(fs.ReadFile(cwd + "/resources/mailinglist.js"))

	}
	endpoint := config.UrlFor("api", "/mailinglist/", m.Id(), "/subscribe")
	return fmt.Sprintf(jsTemplate, endpoint, m.ThankYou, m.Facebook.Id, m.Facebook.Value, m.Facebook.Currency, m.Google.Category, m.Google.Name)
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
