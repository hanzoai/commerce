package discount

import (
	"time"

	"crowdstart.com/models/discount/rule"
	"crowdstart.com/models/discount/scope"
	"crowdstart.com/models/discount/target"
	"crowdstart.com/models/mixin"
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

// Encompasses a given rule trigger and discount amount
type Rule struct {
	// Condition which triggers this rule
	Trigger rule.Trigger `json:"trigger"`

	// Action which happens as result of trigger
	Action rule.Action `json:"action"`
}

type Discount struct {
	mixin.Model

	Name string `json:"name"`

	// Type of discount rule
	Type Type `json:"type"`

	// Date range in which discount is valid
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`

	Scope struct {
		// The scope these rules qualify against
		Type scope.Type `json:"type"`

		// Id for this rule
		StoreId      string `json:"storeId,omitempty"`
		CollectionId string `json:"collectionId,omitempty"`
		ProductId    string `json:"productId,omitempty"`
		VariantId    string `json:"variantId,omitempty"`
	} `json:"scope"`

	Target struct {
		// Target for which all rules apply
		Type target.Type `json:"type"`

		// Id for the target
		ProductId string `json:"productId,omitempty"`
		VariantId string `json:"variantId,omitempty"`
	} `json:"target"`

	// Rules for this discount
	Rules []Rule `json:"rules"`

	// Whether discount is enabled.
	Enabled bool `json:"enabled"`
}

func (d Discount) ValidFor(t time.Time) bool {
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
	if d.Scope.StoreId != "" {
		return d.Scope.StoreId
	}
	if d.Scope.CollectionId != "" {
		return d.Scope.CollectionId
	}
	if d.Scope.ProductId != "" {
		return d.Scope.ProductId
	}
	if d.Scope.VariantId != "" {
		return d.Scope.ProductId
	}
	return ""
}
