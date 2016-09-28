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

	UserId   string `json:"userId"`
	Name     string `json:"name"`
	Company  string `json:"company"`
	Country  string `json:"country"`
	TaxId    string `json:"taxId"`
	Timezone string `json:"timezone"`

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
