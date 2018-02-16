package pricing

import (
	"google.golang.org/appengine"
	aeds "appengine/datastore"

	"hanzo.io/models/types/commission"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/hashid"
)

// Partner pricing fees
type Partner struct {
	Id string `json:"id"`

	// Commission partner receives
	Card struct {
		Commission commission.Commission `json:"commission"`
	} `json:"card"`

	Bitcoin struct {
		Commission commission.Commission `json:"commission"`
	} `json:"bitcoin"`

	Ethereum struct {
		Commission commission.Commission `json:"commission"`
	} `json:"ethereum"`
}

func (p Partner) Key(ctx context.Context) *aeds.Key {
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

	Bitcoin struct {
		Percent float64        `json:"percent,omitempty"`
		Flat    currency.Cents `json:"flat,omitempty"`
	} `json:"bitcoin"`

	Ethereum struct {
		Percent float64        `json:"percent,omitempty"`
		Flat    currency.Cents `json:"flat,omitempty"`
	} `json:"ethereum"`

	// Affiliate fees
	Affiliate struct {
		Percent float64        `json:"percent,omitempty"`
		Flat    currency.Cents `json:"flat,omitempty"`
	} `json:"affiliate"`
}

func (f Fees) Key(ctx context.Context) *aeds.Key {
	key, err := hashid.DecodeKey(ctx, f.Id)
	if err != nil {
		panic(err)
	}
	return key
}
