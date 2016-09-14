package partner

import (
	"time"

	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/commission"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/thirdparty/stripe/connect"

	. "crowdstart.com/models"
)

type Partner struct {
	mixin.Model

	Enabled   bool `json:"enabled"`
	Connected bool `json:"connected"`

	Name     string  `json:"name"`
	Email    string  `json:"email,omitempty"`
	Phone    string  `json:"phone,omitempty"`
	Address  Address `json:"address,omitempty"`
	Website  string  `json:"website,omitempty"`
	Country  string  `json:"country"`
	TaxId    string  `json:"taxId"`
	Timezone string  `json:"timezone"`

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
