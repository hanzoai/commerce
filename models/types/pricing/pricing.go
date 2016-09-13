package pricing

import (
	"appengine"
	aeds "appengine/datastore"

	"crowdstart.com/models/types/commission"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/hashid"
)

// Partner pricing fees
type Partner struct {
	Id string `json:"id"`

	// Commission partner receives
	Commission commission.Commission `json:"commission"`
}

func (p Partner) Key(ctx appengine.Context) *aeds.Key {
	key, err := hashid.DecodeKey(ctx, p.Id)
	if err != nil {
		panic(err)
	}
	return key
}

// Various fees we collect
type Fees struct {
	Id string `json:"id"`

	// Debit/Credit Card processing fees
	Card struct {
		Percent       float64        `json:"percent,omitempty"`
		Flat          currency.Cents `json:"flat,omitempty"`
		Amex          float64        `json:"amex,omitempty"`
		International float64        `json:"international,omitempty"`
	} `json:"card"`

	// Affiliate fees
	Affiliate struct {
		Percent float64        `json:"percent,omitempty"`
		Flat    currency.Cents `json:"flat,omitempty"`
	} `json:"affiliate"`
}

func (f Fees) Key(ctx appengine.Context) *aeds.Key {
	key, err := hashid.DecodeKey(ctx, f.Id)
	if err != nil {
		panic(err)
	}
	return key
}
