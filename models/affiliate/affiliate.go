package affiliate

import (
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/commission"
	"github.com/hanzoai/commerce/models/types/schedule"
	"github.com/hanzoai/commerce/thirdparty/stripe/connect"
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
	CouponId   string                `json:"couponId,omitempty"`

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
