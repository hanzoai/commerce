package order

import (
	"errors"
	"fmt"

	"hanzo.io/models/store"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/log"
)

// Calculates order totals
func (o *Order) Tally() {
	log.Debug("Tallying up order")

	// Update total
	linetotal := 0
	for _, item := range o.Items {
		linetotal += item.Quantity * int(item.Price)
	}
	o.LineTotal = currency.Cents(linetotal)
	o.Subtotal = o.LineTotal - o.Discount
	o.Total = o.Subtotal + o.Tax + o.Shipping
}

// Update order with information from datastore and tally
func (o *Order) UpdateAndTally(stor *store.Store) error {
	// Taxless
	useFallback := false
	if stor == nil {
		useFallback = true
		log.Warn("Fallback: Using client tax and shipping values.", o.Context())
	}

	ctx := o.Context()

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

	// Get underlying product/variant entities
	log.Debug("Fetching underlying line items")
	if err := o.GetItemEntities(); err != nil {
		log.Error(err, ctx)
		return errors.New("Failed to get all underlying line items")
	}

	// Update against store listings
	log.Debug("Updating items against store listing")
	o.UpdateEntities(stor)

	// Update line items using that information
	log.Debug("Updating line items")
	o.UpdateFromEntities()

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

	// If not using fallback mode, skip taxes
	if !useFallback {
		if trs, err := stor.GetTaxRates(); trs == nil {
			log.Warn("Failed to get taxrates for discount rules: %v", err, ctx)
		} else {
		}

		if srs, err := stor.GetShippingRates(); srs == nil {
			log.Warn("Failed to get shippingrates for discount rules: %v", err, ctx)
		} else {
		}
	}

	// Tally up order again
	o.Tally()

	return nil
}
