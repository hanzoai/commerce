package affiliate

import (
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/user"
	"crowdstart.com/thirdparty/stripe/connect"
	"crowdstart.com/util/val"
)

type FeeType string

const (
	Percent FeeType = "percent"
	Flat            = "flat"
)

type Affiliate struct {
	mixin.Model

	Enabled bool `json:"enabled"`

	UserId    string `json:"userId"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Company   string `json:"company"`
	Country   string `json:"country"`
	TaxId     string `json:"-"`
	Timezone  string `json:"timezone"`

	Fee struct {
		Type    FeeType        `json:"feeType"`
		Percent float64        `json:"percent,omitempty"`
		Flat    currency.Cents `json:"flat,omitempty"`
	} `json:"fee"`

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
