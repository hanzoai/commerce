package partner

import (
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/commission"
	"hanzo.io/models/types/schedule"
	"hanzo.io/thirdparty/stripe/connect"

	. "hanzo.io/models"
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
	Schedule   schedule.Schedule     `json:"schedule"`

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
