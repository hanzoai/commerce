package order

import (
	"errors"
	"fmt"

	"crowdstart.com/models/store"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/log"
)

// Calculates order totals
func (o *Order) Tally() {
	log.Debug("Tallying up order")

	// Update total
	subtotal := 0
	nItems := len(o.Items)
	for i := 0; i < nItems; i++ {
		subtotal += o.Items[i].Quantity * int(o.Items[i].Price)
	}
	o.LineTotal = currency.Cents(subtotal)

	// TODO: Make this use shipping/tax information
	discount := int(o.Discount)
	shipping := int(o.Shipping)
	tax := int(o.Tax)
	subtotal = subtotal - discount
	total := subtotal + tax + shipping

	o.Subtotal = currency.Cents(subtotal)
	o.Total = currency.Cents(total)
}

// Update order with information from datastore and tally
func (o *Order) UpdateAndTally(stor *store.Store) error {
	ctx := o.Db.Context

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
	if stor != nil {
		o.UpdateEntities(stor)
	}

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

	// Tally up order again
	o.Tally()

	return nil
}
