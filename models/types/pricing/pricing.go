package pricing

import (
	"context"

	"github.com/hanzoai/commerce/datastore/iface"
	"github.com/hanzoai/commerce/datastore/key"
	"github.com/hanzoai/commerce/models/types/commission"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/hashid"
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

func (p Partner) Key(ctx context.Context) iface.Key {
	dbKey, err := hashid.DecodeKey(ctx, p.Id)
	if err != nil {
		panic(err)
	}
	return key.FromDBKey(dbKey)
}

// CryptoDeposit is what ONE org is charged to RECEIVE a crypto deposit on one
// chain. Keyed by the deposit rail's own chain names ("base", "ethereum",
// "solana", "ton", "xrpl") on Organization.CryptoDeposit.
//
// It is DELIBERATELY NOT Fees.Bitcoin / Fees.Ethereum, which look like the same
// thing and are not. Those are the platform fee on an ORDER priced in BTC or
// ETH — a customer buying something — and order.go applies them to the order
// total. A deposit is the opposite direction: somebody sending us coin to top
// up. Sharing one field would make an org's order fee silently become its
// deposit fee, and neither number was agreed for the other purpose.
//
// The zero value deducts NOTHING, which is what an org with no negotiated terms
// should get; the platform default then applies instead. Both fields mirror
// depositwatch.Terms exactly, because a second spelling of the same two numbers
// is a second thing to keep in step.
type CryptoDeposit struct {
	// FeeCents is what it costs US to sweep the deposit out of the custody
	// address — flat, in whole cents. Not the sender's own network fee.
	FeeCents int64 `json:"feeCents,omitempty"`
	// SlippageBps is a haircut against the price moving between the rate quoted
	// and the rate credited. Meaningful only for a market-priced coin; a
	// stablecoin is credited at a fixed peg and the rail refuses a haircut there.
	SlippageBps int32 `json:"slippageBps,omitempty"`
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

func (f Fees) Key(ctx context.Context) iface.Key {
	dbKey, err := hashid.DecodeKey(ctx, f.Id)
	if err != nil {
		panic(err)
	}
	return key.FromDBKey(dbKey)
}
