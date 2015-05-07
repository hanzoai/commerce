package organization

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryanuber/go-glob"

	"appengine"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models/user"
	"crowdstart.io/thirdparty/stripe/connect"
	"crowdstart.io/util/permission"
	"crowdstart.io/util/val"

	. "crowdstart.io/models"
)

type Organization struct {
	mixin.Model
	mixin.AccessToken

	Name       string   `json:"name"`
	FullName   string   `json:"fullName"`
	Owners     []string `json:"owners,omitempty"`
	Admins     []string `json:"admins,omitempty"`
	Moderators []string `json:"moderators,omitempty"`
	Enabled    bool     `json:"enabled"`

	BillingEmail string  `json:"billingEmail,omitempty"`
	Phone        string  `json:"phone,omitempty"`
	Address      Address `json:"address,omitempty"`
	Website      string  `json:"website,omitempty"`

	Timezone string `json:"timezone"`

	Country string `json:"country"`
	TaxId   string `json:"-"`

	Plan struct {
		PlanId    string
		StartDate time.Time
	} `json:"-"`

	Salesforce struct {
		AccessToken        string `json:"accessToken"`
		DefaultPriceBookId string `json:"defaultPriceBookId"`
		// personalized login url
		Id           string `json:"id"`
		InstanceUrl  string `json:"instanceUrl"`
		IssuedAt     string `json:"issuedAt"`
		RefreshToken string `json:"refreshToken"`
		Signature    string `json:"signature"`
	} `json:"-"`

	Stripe struct {
		// For convenience duplicated
		AccessToken    string
		PublishableKey string
		RefreshToken   string
		UserId         string

		// Save entire live and test tokens
		Live connect.Token
		Test connect.Token
	} `json:"-"`

	GoogleAnalytics string `json:"googleAnalytics"`
	FacebookTag     string `json:"facebookTag"`

	// Whether we use live or test tokens, mostly applicable to stripe
	Live bool `json:"-" datastore:"-"`

	// List of comma deliminated email globs that result in charges of 50 cents
	EmailWhitelist string `json:"emailWhitelist"`
}

func New(db *datastore.Datastore) *Organization {
	o := new(Organization)
	o.Model = mixin.Model{Db: db, Entity: o}
	o.AccessToken = mixin.AccessToken{Entity: o}
	o.Admins = make([]string, 0)
	o.Moderators = make([]string, 0)
	return o
}

func (o Organization) Kind() string {
	return "organization"
}

func (o *Organization) Validator() *val.Validator {
	return val.New(o).Check("FullName").Exists()
}

func (o *Organization) AddDefaultTokens() {
	o.RemoveToken("live-secret-key")
	o.RemoveToken("live-published-key")
	o.RemoveToken("test-secret-key")
	o.RemoveToken("test-published-key")
	o.AddToken("live-secret-key", permission.Admin|permission.Live)
	o.AddToken("live-published-key", permission.Published|permission.Live)
	o.AddToken("test-secret-key", permission.Admin|permission.Test)
	o.AddToken("test-published-key", permission.Published|permission.Test)
}

func (o Organization) IsAdmin(user *user.User) bool {
	for _, userId := range o.Admins {
		if userId == user.Id() {
			return true
		}
	}
	return false
}

func (o Organization) IsOwner(user *user.User) bool {
	for _, userId := range o.Owners {
		if userId == user.Id() {
			return true
		}
	}
	return false
}

func (o Organization) Namespace(ctx interface{}) appengine.Context {
	var _ctx appengine.Context

	switch v := ctx.(type) {
	case *gin.Context:
		_ctx = v.MustGet("appengine").(appengine.Context)
	case appengine.Context:
		_ctx = v
	}

	_ctx, err := appengine.Namespace(_ctx, o.Name)
	if err != nil {
		panic(err)
	}
	return _ctx
}

func (o Organization) StripeToken() string {
	if o.Live {
		return o.Stripe.Live.AccessToken
	}

	return o.Stripe.Test.AccessToken
}

func (o Organization) IsTestEmail(email string) bool {
	if o.EmailWhitelist == "" {
		return false
	}

	globs := strings.Split(o.EmailWhitelist, ",")

	for _, g := range globs {
		if glob.Glob(g, email) {
			return true
		}
	}

	return false
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
