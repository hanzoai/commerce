package discount

import (
	"time"

	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/log"
	"crowdstart.com/util/timeutil"
)

type Type string

const (
	Flat         Type = "flat"
	Percent           = "percent"
	FreeShipping      = "free-shipping"
	FreeItem          = "free-item"
	Bulk              = "bulk"
)

var Types = []Type{Flat, Percent, FreeShipping, FreeItem, Bulk}

type ScopeType string

const (
	Organization ScopeType = "organization"
	Product                = "product"
	Collection             = "collection"
	Store                  = "store"
	Variant                = "variant"
)

type Rule struct {
	// Range in which this discount is active
	Range struct {
		// Quantity range which triggers this rule
		Quantity struct {
			Start int `json:"start,omitempty"`
			End   int `json:"end,omitempty"`
		} `json:"quantity,omitempty"`

		// Price range which triggers this rule
		Price struct {
			Start currency.Cents `json:"start,omitempty"`
			End   currency.Cents `json:"end,omitempty"`
		} `json:"price,omitempty"`
	} `json:"range"`

	// Amount of discount
	Amount struct {
		Flat    int     `flat,omitempty`
		Percent float64 `percent,omitempty`
	} `json:"amount"`
}

type Discount struct {
	mixin.Model

	Name string `json:"name"`

	// Type of discount rule
	Type Type `json:"type"`

	// Date range in which discount is valid
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`

	// Scope this rule applies to
	Scope ScopeType `json:"scope"`

	// Id for this rule
	StoreId      string `json:"storeId,omitempty"`
	CollectionId string `json:"collectionId,omitempty"`
	ProductId    string `json:"productId,omitempty"`
	VariantId    string `json:"variantId,omitempty"`

	// Rules for this discount
	Rules []Rule `json:"rules"`

	// Whether discount is enabled.
	Enabled bool `json:"enabled"`
}

func (d Discount) Valid(t time.Time) bool {
	ctx := d.Context()
	if !d.Enabled {
		log.Warn("Discount Not Enabled", ctx)
		return false // currently active, no need to check?
	}

	if !timeutil.IsZero(d.StartDate) && d.StartDate.After(t) {
		log.Warn("Discount not yet Usable: %v > %v", d.StartDate.Unix(), t, ctx)
		return false
	}

	if !timeutil.IsZero(d.EndDate) && d.EndDate.Before(t) {
		log.Warn("Discount is Expired: %v < %v", d.EndDate.Unix(), t, ctx)
		return false
	}

	return true
}

func (d Discount) ScopeId() string {
	if d.StoreId != "" {
		return d.StoreId
	}
	if d.CollectionId != "" {
		return d.CollectionId
	}
	if d.ProductId != "" {
		return d.ProductId
	}
	if d.VariantId != "" {
		return d.ProductId
	}
	return ""
}
