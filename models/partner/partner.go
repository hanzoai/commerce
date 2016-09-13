package partner

import (
	"time"

	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/commission"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/thirdparty/stripe/connect"
)

type Partner struct {
	mixin.Model

	Connected bool `json:"connected"`
	Enabled   bool `json:"enabled"`

	UserId    string `json:"userId"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Company   string `json:"company"`
	Country   string `json:"country"`
	TaxId     string `json:"-"`
	Timezone  string `json:"timezone"`

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
