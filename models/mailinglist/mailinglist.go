package mailinglist

import (
	"fmt"
	"os"

	"google.golang.org/appengine"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/subscriber"
	"hanzo.io/models/types/form"
	"hanzo.io/models/types/thankyou"
	"hanzo.io/util/fs"
	"hanzo.io/util/json"
	"hanzo.io/util/val"
)

var jsTemplate = ""

// Settings used for injection into form.js
type Settings struct {
	// Name of list
	Name string `json:"name"`

	// Type of form
	Type form.Type `json:"type"`

	// Thank you settings
	ThankYou ThankYou `json:"thankyou"`
}

// Thank you configuration
type ThankYou struct {
	Type thankyou.Type `json:"type"`
	Url  string        `json:"url,omitempty"`
	HTML string        `json:"html,omitempty"`
}

// Mailchimp configuration
type MailChimp struct {
	ListId           string `json:"listId"`
	APIKey           string `json:"apiKey"`
	DoubleOptin      bool   `json:"doubleOptin"`
	UpdateExisting   bool   `json:"updateExisting"`
	ReplaceInterests bool   `json:"replaceInterests"`

	// Whether to have Mailchimp send email confirmation
	SendWelcome bool `json:"sendWelcome"`

	Enabled bool `json:"enabled"`
}

type MailingList struct {
	mixin.Model

	// Name of list
	Name string `json:"name"`

	// Type of form
	Type form.Type `json:"type"`

	// Whether to send email confirmation
	SendWelcome bool `json:"sendWelcome"`

	// Mailchimp settings for this list
	Mailchimp MailChimp `json:"mailchimp,omitempty"`

	// Email forwarding
	Forward struct {
		Email   string `json:"email"`
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	} `json:"forward"`

	// Thank you settings
	ThankYou ThankYou `json:"thankyou"`

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

func (m *MailingList) Validator() *val.Validator {
	return val.New()
}

func (m *MailingList) AddSubscriber(s *subscriber.Subscriber) error {
	mkey := m.Key()
	s.MailingListId = m.Id()
	s.Parent = mkey
	s.Normalize()

	return m.Db.RunInTransaction(func(db *datastore.Datastore) error {
		keys, err := subscriber.Query(db).Ancestor(mkey).Filter("Email=", s.Email).GetKeys()

		if len(keys) != 0 {
			return SubscriberAlreadyExists
		}

		if err != nil {
			return err
		}

		return s.Create()
	}, nil)
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
	} else {
		endpoint = "https:" + endpoint
	}

	return fmt.Sprintf(jsTemplate, endpoint, json.Encode(Settings{m.Name, m.Type, m.ThankYou}))
}

func FromJSON(db *datastore.Datastore, data []byte) *MailingList {
	ml := New(db)
	json.DecodeBytes(data, ml)
	return ml
}
