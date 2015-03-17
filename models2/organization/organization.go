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

	Name       string
	FullName   string
	Owners     []string
	Admins     []string
	Moderators []string
	Enabled    bool

	BillingEmail string
	Phone        string
	Address      Address
	Website      string

	Timezone string

	Country string
	TaxId   string

	Plan struct {
		PlanId    string
		StartDate time.Time
	}

	Salesforce struct {
		AccessToken        string
		DefaultPriceBookId string
		Id                 string // personalized login url
		InstanceUrl        string
		IssuedAt           string
		RefreshToken       string
		Signature          string
	}

	Stripe struct {
		AccessToken    string
		Livemode       bool
		PublishableKey string
		RefreshToken   string
		Scope          string
		TokenType      string
		UserId         string
	}

	GoogleAnalytics string
	FacebookTag     string
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
