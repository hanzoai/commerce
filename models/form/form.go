package form

import (
	"fmt"
	"os"

	"google.golang.org/appengine"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/subscriber"
	"github.com/hanzoai/commerce/models/types/form"
	"github.com/hanzoai/commerce/models/types/thankyou"
	"github.com/hanzoai/commerce/types/email"
	"github.com/hanzoai/commerce/util/fs"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/val"
)

var jsTemplate = ""
var Submit = form.Submit
var Subscribe = form.Subscribe

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

type Form struct {
	mixin.Model

	// Name of list
	Name string `json:"name"`

	// Type of form
	Type form.Type `json:"type"`

	// Whether to send email confirmation
	SendWelcome bool `json:"sendWelcome"`

	// Overwrites default Template Id
	WelcomeTemplateId string `json:"welcomeTemplateId"`

	// Email list settings for this list
	EmailList email.List `json:"emailList,omitempty"`

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

	Mailchimp struct {
		ListId string `json:"listId"`
		APIKey string `json:"apiKey"`
	} `json:"mailchimp,omitempty"`
}

func (f *Form) Validator() *val.Validator {
	return val.New()
}

func (f *Form) AddSubscriber(s *subscriber.Subscriber) error {
	fkey := f.Key()
	s.FormId = f.Id()
	s.Parent = fkey
	s.Normalize()

	return f.Db.RunInTransaction(func(db *datastore.Datastore) error {
		keys, err := subscriber.Query(db).Ancestor(fkey).Filter("Email=", s.Email).GetKeys()

		if len(keys) != 0 {
			return SubscriberAlreadyExists
		}

		if err != nil {
			return err
		}

		return s.Create()
	}, nil)
}

func (f *Form) Js() string {
	if jsTemplate == "" {
		var cwd, _ = os.Getwd()
		jsTemplate = string(fs.ReadFile(cwd + "/resources/form.js"))

	}

	// Endpoint for subscription
	endpoint := config.UrlFor("api", "/form/", f.Id(), "/subscribe")
	if appengine.IsDevAppServer() {
		endpoint = "http://localhost:8080" + endpoint
	} else {
		endpoint = "https:" + endpoint
	}

	return fmt.Sprintf(jsTemplate, endpoint, json.Encode(Settings{f.Name, f.Type, f.ThankYou}))
}

func FromJSON(db *datastore.Datastore, data []byte) *Form {
	f := New(db)
	json.DecodeBytes(data, f)
	return f
}
