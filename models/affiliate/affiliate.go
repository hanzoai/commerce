package affiliate

import (
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/commission"
	"crowdstart.com/models/types/schedule"
	"crowdstart.com/thirdparty/stripe/connect"
)

type Affiliate struct {
	mixin.Model

	Enabled   bool `json:"enabled"`
	Connected bool `json:"connected"`

	UserId   string `json:"userId,omitempty"`
	Name     string `json:"name,omitempty"`
	Company  string `json:"company,omitempty"`
	Country  string `json:"country,omitempty"`
	TaxId    string `json:"taxId,omitempty"`
	Timezone string `json:"timezone,omitempty"`

	Commission commission.Commission `json:"commission,omitempty"`
	Schedule   schedule.Schedule     `json:"schedule,omitempty"`

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
