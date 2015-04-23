package mailinglist

import (
	"fmt"
	"os"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/subscriber"
	"crowdstart.io/util/fs"
	"crowdstart.io/util/log"
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

type ThankYou string

const (
	Html     ThankYou = "html"
	Redirect          = "redirect"
	Disabled          = "disabled"
)

type MailingList struct {
	mixin.Model

	// Name of list
	Name string `json:"name"`

	// Mailchimp settings for this list
	Mailchimp Mailchimp `json:"mailchimp"`

	// Url to Thank you page
	ThankYou struct {
		Type ThankYou `json:"type"`
		Url  string   `json:"url,omitempty"`
		HTML string   `json:"html,omitempty"`
	} `json:"thankyou"`

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
	m.Init()
	m.Model = mixin.Model{Db: db, Entity: m}
	return m
}

func (m *MailingList) Init() {
	m.Facebook.Value = "0.00"
	m.Facebook.Currency = "USD"
	m.ThankYou.Type = Disabled
}

func (m MailingList) Kind() string {
	return "mailinglist"
}

func (m *MailingList) Validator() *val.Validator {
	return val.New(m)
}

func (m *MailingList) AddSubscriber(s *subscriber.Subscriber) error {
	mkey := m.Key()
	s.MailingListId = m.Id()
	s.Parent = mkey

	return m.RunInTransaction(func() error {
		keys, err := subscriber.Query(m.Db).Ancestor(mkey).Filter("Email=", s.Email).KeysOnly().GetAll(nil)
		log.Debug("keys: %v, err: %v", keys, err)

		if len(keys) != 0 {
			return SubscriberAlreadyExists
		}

		if err != nil {
			return err
		}

		return s.Put()
	})
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
