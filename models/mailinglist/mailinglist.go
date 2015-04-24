package mailinglist

import (
	"fmt"
	"os"

	"appengine"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models/subscriber"
	"crowdstart.io/util/fs"
	"crowdstart.io/util/json"
	"crowdstart.io/util/val"
)

var jsTemplate = ""

type Mailchimp struct {
	Id               string `json:"id"`
	APIKey           string `json:"apiKey"`
	DoubleOptin      bool   `json:"doubleOptin"`
	UpdateExisting   bool   `json:"updateExisting"`
	ReplaceInterests bool   `json:"replaceInterests"`

	// Whether to have mailchimp email confirmation
	SendWelcome bool `json:"sendWelcome"`
}

type ThankYou string

const (
	Html     ThankYou = "html"
	Redirect          = "redirect"
	Disabled          = "disabled"
)

var ThankYouTypes = []ThankYou{Html, Redirect, Disabled}

type MailingList struct {
	mixin.Model

	// Name of list
	Name string `json:"name"`

	// Whether to send email confirmation
	SendWelcome bool `json:"sendWelcome"`

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

	// Endpoint for subscription
	endpoint := config.UrlFor("api", "/mailinglist/", m.Id(), "/subscribe")
	if appengine.IsDevAppServer() {
		endpoint = "http://localhost:8080" + endpoint
	}

	return fmt.Sprintf(jsTemplate, endpoint, m.JSON())
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}

func FromJSON(db *datastore.Datastore, data []byte) *MailingList {
	ml := New(db)
	json.DecodeBytes(data, ml)
	return ml
}
