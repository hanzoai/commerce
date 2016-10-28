package referrer

import (
	"crowdstart.com/models/types/currency"
)

type Type string

const (
	StoreCredit Type = "Credit" // Add credit to user's balance
	Refund           = "Refund" // Refund part of the payment on a order
	EmailUser        = "EmailUser"
)

type Credit struct {
	Currency currency.Type  `json:"currency,omitempty"`
	Amount   currency.Cents `json:"amount,omitempty"`
}

type Percent struct {
	Percent float64 `json:"percent,omitempty"`
}

type Trigger struct {
	// A MinReferrals value of 0 indicates that the associated action is
	// always eligible for execution.
	MinReferrals int `json:minreferrals,omitempty`
}

type Action struct {
	Trigger Trigger `json:"trigger"`
	Type Type `json:"type"`
	Credit
	Percent
}

type Program struct {
	Name string `json:"name"`

	Actions  []Action `json:"actions"`
}
