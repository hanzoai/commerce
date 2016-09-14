package affiliate

import (
	"time"

	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/commission"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/thirdparty/stripe/connect"
	"crowdstart.com/util/val"
)

type Affiliate struct {
	mixin.Model

	Enabled   bool `json:"enabled"`
	Connected bool `json:"connected"`

	UserId   string `json:"userId"`
	Name     string `json:"name"`
	Company  string `json:"company"`
	Country  string `json:"country"`
	TaxId    string `json:"taxId"`
	Timezone string `json:"timezone"`

	Commission commission.Commission `json:"commission"`
	Period     int                   `json:"period"`

	LastPaid  time.Time      `json:"lastPaid,omitempty"`
	TotalPaid currency.Cents `json:"totalPaid"`

	Stripe struct {
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
