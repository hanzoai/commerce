package organization

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/user"
	"crowdstart.io/util/permission"
	"crowdstart.io/util/val"

	. "crowdstart.io/models2"
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
		AccessToken    string `json:"accessToken"`
		Livemode       bool   `json:"livemode"`
		PublishableKey string `json:"publishableKey"`
		RefreshToken   string `json:"refreshToken"`
		Scope          string `json:"scope"`
		TokenType      string `json:"tokenType"`
		UserId         string `json:"userId"`
	} `json:"-"`

	GoogleAnalytics string `json:"googleAnalytics"`
	FacebookTag     string `json:"facebookTag"`
}

func New(db *datastore.Datastore) *Organization {
	o := new(Organization)
	o.Model = mixin.Model{Db: db, Entity: o}
	o.AccessToken = mixin.AccessToken{Model: o}
	o.Admins = make([]string, 0)
	o.Moderators = make([]string, 0)
	return o
}

func (o Organization) Kind() string {
	return "organization2"
}

func (o *Organization) Validator() *val.Validator {
	return val.New(o).Check("FullName").Exists()
}

func (o *Organization) AddDefaultTokens() {
	o.AddToken("live-secret-key", permission.Admin)
	o.AddToken("live-published-key", permission.Published)
	o.AddToken("test-secret-key", permission.Admin)
	o.AddToken("test-published-key", permission.Published)
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

	_ctx, err := appengine.Namespace(_ctx, strconv.Itoa(int(o.Key().IntID())))
	if err != nil {
		panic(err)
	}
	return _ctx
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
