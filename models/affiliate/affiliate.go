package affiliate

import (
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/user"
	"crowdstart.com/thirdparty/stripe/connect"
	"crowdstart.com/util/val"

	. "crowdstart.com/models"
)

type Affiliate struct {
	mixin.Model
	mixin.AccessToken

	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Enabled   bool   `json:"enabled"`

	Email   string  `json:"billingEmail,omitempty"`
	Phone   string  `json:"phone,omitempty"`
	Address Address `json:"address,omitempty"`
	Website string  `json:"website,omitempty"`

	Timezone string `json:"timezone"`

	Country string `json:"country"`
	TaxId   string `json:"-"`

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

	Mandrill struct {
		APIKey string
	} `json:"-"`

	// Whether we use live or test tokens, mostly applicable to stripe
	Live bool `json:"-" datastore:"-"`
}

func (a Affiliate) GetStripeAccessToken(userId string) (string, error) {
	if a.Stripe.Live.UserId == userId {
		return a.Stripe.Live.AccessToken, nil
	}
	if a.Stripe.Test.UserId == userId {
		return a.Stripe.Test.AccessToken, nil
	}
	return "", StripeAccessTokenNotFound{userId, a.Stripe.Live.UserId, a.Stripe.Test.UserId}
}

func (a *Affiliate) Validator() *val.Validator {
	return val.New().Check("Email").Exists()
}

func userId(userOrId interface{}) string {
	userid := ""
	switch v := userOrId.(type) {
	case *user.User:
		userid = v.Id()
	case string:
		userid = v
	}
	return userid
}

func (a Affiliate) StripeToken() string {
	if a.Live {
		return a.Stripe.Live.AccessToken
	}

	return a.Stripe.Test.AccessToken
}
