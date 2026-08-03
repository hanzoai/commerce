package shipping

import (
	"sort"

	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/types/weight"
	"github.com/hanzoai/money"
)

type RateType string

const (
	Flat     RateType = "flat"
	Variable          = "variable"
)

// This represents the minimum value of a shipping formula
//
//	for example: Shipping = $10 Flat Rate if weight > 10 lbs
type Formula struct {
	MinWeight weight.Mass    `json:"minWeight"`
	RateType  RateType       `json:"type"`
	Price     currency.Cents `json:"price"`
}

// A collection of shipping rate formulas, all must have a common weight and currency unit
type Rates struct {
	Formulas   []Formula     `json:"formulas"`
	WeightUnit weight.Unit   `json:"weightUnit"`
	Currency   currency.Type `json:"currency"`

	// Rate used by default
	BaseRateType RateType       `json:"type"`
	BasePrice    currency.Cents `json:"price"`
}

func (r Rates) Len() int {
	return len(r.Formulas)
}

func (r *Rates) Swap(i, j int) {
	r.Formulas[i], r.Formulas[j] = r.Formulas[j], r.Formulas[i]
}

func (r Rates) Less(i, j int) bool {
	return r.Formulas[i].MinWeight < r.Formulas[j].MinWeight
}

// To calculate shipping rate, we sort an array of formulas by the MinWeight ascending
// When we find the first element with a weight greater than a min, we calculate using the previous one
func (r Rates) GetPrice(p *product.Product) (currency.Cents, currency.Type) {
	// Convert to f units
	w := weight.Convert(p.Weight, p.WeightUnit, r.WeightUnit)

	sort.Sort(&r)

	// i is index of last Formula compared
	i := -1
	for j, f := range r.Formulas {

		// Break if MinWeight is less than Product Weight
		if w < f.MinWeight {
			break
		}

		// Set index to current formula
		i = j
	}

	if i == -1 {
		// Use the base rate if weight is less than the first MinWeight
		return calculateShippingPrice(w, r.BaseRateType, r.BasePrice), r.Currency
	}
	// Otherwise look up the corresponding formula and use the rates on it
	f := r.Formulas[i]
	return calculateShippingPrice(w, f.RateType, f.Price), r.Currency
}

// helpers
func calculateShippingPrice(w weight.Mass, rateType RateType, price currency.Cents) currency.Cents {
	switch rateType {
	case Variable:
		// A variable rate bills price per unit of weight, so the weight IS the rate.
		// Round up: a part-cent of carriage is charged, not absorbed. That direction is
		// deliberate and unchanged; what changes is that it now rounds the amount instead
		// of the float, which used to bill a cent that was not owed — 0.07kg at 700c/kg is
		// exactly 49c, but float 0.07 is a hair high, so the product came out
		// 49.000000000000007 and ceilinged to 50.
		rate, err := money.RateFromFloat(float64(w))
		if err != nil {
			// weight.Convert scales by 1/grams-per-unit, so a rate row with no WeightUnit
			// makes every weight non-finite. That cannot price anything, and converting it
			// to Cents is undefined in Go — it yielded MaxInt64, MinInt64 or 0 depending on
			// which non-finite it was. The row's own configured price is the defined answer.
			return price
		}
		return price.ScaleCeil(rate)
	default:
		// Flat/other cases
		return price
	}
}
