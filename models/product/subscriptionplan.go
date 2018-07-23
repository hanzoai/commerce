package product

import (
	"hanzo.io/models/types/refs"
	"hanzo.io/models/types/currency"
)

type Interval string

const (
	Yearly  Interval = "year"
	Monthly Interval = "month"
)

type SubscriptionPlan struct {
	Price           currency.Cents `json:"price"`
	MSRP		    currency.Cents `json:"msrp,omitempty"`
	Currency        currency.Type  `json:"currency"`

	Interval        Interval       `json:"interval"`
	IntervalCount   int            `json:"intervalCount"`
	TrialPeriodDays int            `json:"trialPeriodDays"`

	Ref refs.EcommerceRef `json:"ref,omitempty"`
}
