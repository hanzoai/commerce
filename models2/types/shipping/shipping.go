package shipping

import (
	"math"
	"sort"

	"crowdstart.io/models2/product"
	"crowdstart.io/models2/types/currency"
	"crowdstart.io/models2/types/weight"
)

type Type string

const (
	Flat     Type = "flat"
	Variable      = "variable"
)

// This represents the minimum value of a shipping formula
//  for example: Shipping = $10 Flat Rate if weight > 10 lbs
type Formula struct {
	MinWeight weight.Mass
	RateType  Type
	Price     currency.Cents
	Currency  currency.Type
}

// A collection of shipping rate formulas, all must have a common weight unit
type Rates struct {
	Formulas   []Formula
	WeightUnit weight.Unit
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
// When we find the first element with a weight greater than a min, we return the shipping price
func (r Rates) GetCost(p *product.Product) (currency.Cents, currency.Type) {
	w, _ := weight.Convert(p.Weight, p.WeightUnit, r.WeightUnit)

	sort.Sort(&r)
	for _, f := range r.Formulas {
		// Convert to f units

		// Formul
		if w < p.Weight {
			continue
		}

		switch f.RateType {
		case Variable:
			// Do the math and round up for variable rates
			return currency.Cents(math.Ceil(float64(w) * float64(f.Price))), f.Currency
		case Flat:
			return f.Price, f.Currency
		}

		return currency.Cents(0), currency.USD
	}

	return currency.Cents(0), currency.USD
}
