package organization

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/user"

	. "crowdstart.io/models2"
)

type Organization struct {
	mixin.Model
	mixin.AccessToken

	Name       string   `json:"name"`
	FullName   string   `json:"fullName"`
	Owners     []string `json:"owners"`
	Admins     []string `json:"admins"`
	Moderators []string `json:"moderators"`
	Enabled    bool     `json:"enabled"`

	BillingEmail string  `json:"billingEmail"`
	Phone        string  `json:"phone"`
	Address      Address `json:"address"`
	Website      string  `json:"website"`

	Timezone string `json:"timezone"`

	Country string `json:"country"`
	TaxId   string `json:"taxId"`

	Plan struct {
		PlanId    string    `json:"planId"`
		StartDate time.Time `json:"startDate"`
	}

	Salesforce struct {
		AccessToken        string `json:"accessToken"`
		DefaultPriceBookId string `json:"defaultPriceBookId"`
		// personalized login url
		Id           string `json:"id"`
		InstanceUrl  string `json:"instanceUrl"`
		IssuedAt     string `json:"issuedAt"`
		RefreshToken string `json:"refreshToken"`
		Signature    string `json:"signature"`
	}

	Stripe struct {
		AccessToken    string `json:"accessToken"`
		Livemode       bool   `json:"livemode"`
		PublishableKey string `json:"publishableKey"`
		RefreshToken   string `json:"refreshToken"`
		Scope          string `json:"scope"`
		TokenType      string `json:"tokenType"`
		UserId         string `json:"userId"`
	}

	GoogleAnalytics string `json:"googleAnalytics"`
	FacebookTag     string `json:"facebookTag"`
}

func New(db *datastore.Datastore) *Organization {
	o := new(Organization)
	o.Model = mixin.Model{Db: db, Entity: o}
	o.AccessToken = mixin.AccessToken{Model: o}
	return o
}

func (o Organization) Kind() string {
	return "organization2"
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

func (o Organization) GenerateAccessToken(user *user.User) (string, error) {
	if o.IsOwner(user) || o.IsAdmin(user) {
		return o.AccessToken.GenerateAccessToken()
	} else {
		return "", errors.New("User is not authorized to create a new access token.")
	}
}

func (o Organization) Namespace(ctx interface{}) appengine.Context {
	var _ctx appengine.Context

	switch v := ctx.(type) {
	case *gin.Context:
		_ctx = v.MustGet("appengine").(appengine.Context)
	case appengine.Context:
		_ctx = v
	}

	_ctx, err := appengine.Namespace(_ctx, o.Id())
	if err != nil {
		panic(err)
	}
	return _ctx
}
