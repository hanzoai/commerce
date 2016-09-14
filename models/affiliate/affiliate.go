package affiliate

import (
	"time"

	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/commission"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/thirdparty/stripe/connect"
	"crowdstart.com/util/timeutil"
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

func (a *Affiliate) TransferCutoff() time.Time {
	// Figure out payment start
	start := a.LastPaid
	if timeutil.IsZero(start) {
		// FIXME: This should really be first date of first referral or some
		// sort of scheduled start like 1, or 15th.
		start = a.CreatedAt
	}

	// Should payout for transfers on net 20
	year, month, day := start.Date()
	t := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	t = t.AddDate(0, 0, -a.Period)
	return t
}
