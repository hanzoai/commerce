package order

import (
	"errors"
	"fmt"

	"hanzo.io/models/store"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/json"
	"hanzo.io/log"
)

// Calculates order totals
func (o *Order) Tally() {
	o.TallySubtotal()
	o.TallyTotal()
}

func (o *Order) TallySubtotal() {
	// Contributions do not have items
	if o.Mode == DepositMode || o.Mode == ContributionMode || o.TokenSaleId != "" {
		return
	}

	log.Debug("Tallying up order subtotal")

	// Update total
	linetotal := 0
	for _, item := range o.Items {
		linetotal += item.Quantity * int(item.Price)
	}
	o.LineTotal = currency.Cents(linetotal)
	o.Subtotal = o.LineTotal - o.Discount
}

func (o *Order) TallyTotal() {
	log.Debug("Tallying up order total")
	o.Total = o.Subtotal + o.Tax + o.Shipping
}

// Update order with information from datastore and tally
func (o *Order) UpdateAndTally(stor *store.Store) error {
	ctx := o.Context()

	// Taxless
	useFallback := false
	if stor == nil {
		useFallback = true
		log.Warn("Fallback: Using client tax and shipping values.", ctx)
	}

	// Get coupons from datastore
	log.Debug("Getting coupons for order")
	if err := o.GetCoupons(); err != nil {
		log.Error(err, ctx)
		return errors.New("Failed to get coupons")
	}

	log.Debug("Checking for redeemed coupons")
	for _, coup := range o.Coupons {
		if !coup.Redeemable() {
			return errors.New(fmt.Sprintf("Coupon %v limit reached", coup.Code()))
		}
	}

	// Update the list of free coupon items
	log.Debug("Add free items from coupons")
	o.UpdateCouponItems()

	log.Info("Order Mode: '%v'\nTokenSaleId: '%s'", o.Mode, o.TokenSaleId, ctx)
	// Tokensales and contributions have no items
	if o.Mode != DepositMode && o.Mode != ContributionMode && o.TokenSaleId == "" {
		// Get underlying product/variant entities
		log.Debug("Fetching underlying line items")
		if err := o.GetItemEntities(); err != nil {
			log.Error(err, ctx)
			return errors.New("Failed to get all underlying line items")
		}
	}

	log.Info("Order Before Updating Entities: '%v'", json.Encode(o.Items), ctx)

	// Update against store listings
	log.Debug("Updating items against store listing")
	if stor != nil {
		o.UpdateEntities(stor)
	}

	log.Info("Order Before Updating From Entities: '%v'", json.Encode(o.Items), ctx)

	// Update line items using that information
	log.Debug("Updating line items")
	o.UpdateFromEntities()

	log.Info("Order After Updating From Entities: '%v'", json.Encode(o.Items), ctx)

	// Calculate applicable discount from discount rules
	log.Debug("Calculating discount from discount rules")
	discount, err := o.CalcRuleDiscount()
	if err != nil {
		log.Warn("Failed to calculate discount from discount rules: %v", err, ctx)
		return err
	}

	// Add applicable coupon discount
	log.Debug("Calculating discount from coupons")
	discount += o.CalcCouponDiscount()

	// Update order total discount
	o.Discount = discount

	// Tally up order again
	o.TallySubtotal()

	// If not using fallback mode, skip taxes
	if !useFallback {
		if stor.Currency != "" {
			o.Currency = stor.Currency
		}

		o.Shipping = 0

		// Tax may depend on shipping so calcualte that first
		if srs, err := stor.GetShippingRates(); srs == nil {
			log.Warn("Failed to get shippingrates for discount rules: %v", err, ctx)
		} else if match, _, _ := srs.Match(o.ShippingAddress.Country, o.ShippingAddress.State, o.ShippingAddress.City, o.ShippingAddress.PostalCode); match != nil {
			o.Shipping = match.Cost + currency.Cents(float64(o.Subtotal)*match.Percent)
		}

		o.Tax = 0

		if trs, err := stor.GetTaxRates(); trs == nil {
			log.Warn("Failed to get taxrates for discount rules: %v", err, ctx)
		} else if match, _, _ := trs.Match(o.ShippingAddress.Country, o.ShippingAddress.State, o.ShippingAddress.City, o.ShippingAddress.PostalCode); match != nil {
			if match.TaxShipping {
				o.Tax = match.Cost + currency.Cents(float64(o.Subtotal+o.Shipping)*match.Percent)
			} else {
				o.Tax = match.Cost + currency.Cents(float64(o.Subtotal)*match.Percent)
			}
		}
	}

	o.TallyTotal()

	return nil
}
