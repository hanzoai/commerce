package organization

import (
	"time"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"

	. "crowdstart.io/models2"
)

type Organization struct {
	mixin.Model

	// Use Name as JWT "iss" param
	Name       string
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

	Crowdstart struct {
		// Use IssuedAt as JWT "iat" param
		IssuedAt time.Time
		// JWT secret
		SecretKey string

		// UseTokenId as JWT "jti" param, randomly generate upon generating a new key to expire all existing keys
		TokenId string
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
	return o
}

func (o Organization) Kind() string {
	return "organization2"
}
