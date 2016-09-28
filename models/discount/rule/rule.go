package rule

import (
	"crowdstart.com/models/discount/action"
	"crowdstart.com/models/discount/trigger"
	"crowdstart.com/models/types/currency"
)

// Quantity-based trigger
type Quantity struct {
	Start int `json:"start,omitempty"`
}

// Price-based trigger
type Price struct {
	Start currency.Cents `json:"start,omitempty"`
}

// Union of possible triggers
type Trigger struct {
	Price    Price    `json:"price,omitempty"`
	Quantity Quantity `json:"quantity,omitempty"`
}

// Determine type of trigger
func (t Trigger) Type() trigger.Type {
	if t.Quantity.Start > 0 {
		return trigger.Quantity
	}

	if t.Price.Start > 0 {
		return trigger.Price
	}

	panic("Invalid trigger type for discount rule")
}

// Discount action
type Discount struct {
	Flat    currency.Cents `flat,omitempty`
	Percent float64        `percent,omitempty`
}

// Union of possible actions
type Action struct {
	Discount
}

// Determine type of action
func (a Action) Type() action.Type {
	return action.Discount
}
