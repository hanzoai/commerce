package order

import (
	"crowdstart.com/models/discount"
	"crowdstart.com/models/discount/scope"
	"crowdstart.com/models/discount/target"
	"crowdstart.com/models/discount/trigger"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/log"
)

// Alias type for simplicity
type discounts []discount.Discount

// Append discounts which are valid for order creation date
func (o *Order) appendValidDiscounts(to discounts, from discounts) discounts {
	for i := 0; i < len(from); i++ {
		if from[i].ValidFor(o.CreatedAt) {
			to = append(to, from[i])
		}
	}
	return to
}

func (o *Order) addOrgDiscounts(disc chan discounts, errc chan error) {
	dst := make(discounts, 0)
	_, err := discount.Query(o.Db).
		Filter("Scope=", scope.Organization).
		Filter("Enabled=", true).
		GetAll(&dst)
	errc <- err
	disc <- dst
}

func (o *Order) addStoreDiscounts(disc chan discounts, errc chan error) {
	dst := make(discounts, 0)
	_, err := discount.Query(o.Db).
		Filter("StoreId=", o.StoreId).
		Filter("Enabled=", true).
		GetAll(&dst)
	errc <- err
	disc <- dst
}

func (o *Order) addCollectionDiscounts(id string, disc chan discounts, errc chan error) {
	dst := make(discounts, 0)
	_, err := discount.Query(o.Db).
		Filter("CollectionId=", id).
		Filter("Enabled=", true).
		GetAll(&dst)
	errc <- err
	disc <- dst
}

func (o *Order) addProductDiscounts(id string, disc chan discounts, errc chan error) {
	dst := make(discounts, 0)
	_, err := discount.Query(o.Db).
		Filter("ProductId=", id).
		Filter("Enabled=", true).
		GetAll(&dst)
	errc <- err
	disc <- dst
}

func (o *Order) addVariantDiscounts(id string, disc chan discounts, errc chan error) {
	dst := make(discounts, 0)
	_, err := discount.Query(o.Db).
		Filter("VariantId=", id).
		Filter("Enabled=", true).
		GetAll(&dst)
	errc <- err
	disc <- dst
}

func (o *Order) GetDiscounts() (discounts, error) {
	channels := 2 + len(o.Items)
	errc := make(chan error, channels)
	disc := make(chan discounts, channels)

	// Fetch any organization-level discounts
	go o.addOrgDiscounts(disc, errc)

	// Fetch any store-level discounts
	go o.addStoreDiscounts(disc, errc)

	// Fetch any product or variant level discounts
	for _, item := range o.Items {
		if item.ProductId != "" {
			go o.addProductDiscounts(item.ProductId, disc, errc)
		} else if item.VariantId != "" {
			go o.addVariantDiscounts(item.VariantId, disc, errc)
		}
	}

	// Check for any query errors
	for i := 0; i < channels; i++ {
		err := <-errc
		if err != nil {
			log.Warn("Unable to fetch all discounts: %v", err, o.Context())
			return nil, err
		}
	}

	// Merge results together
	ret := make(discounts, 0)
	for i := 0; i < channels; i++ {
		dis := <-disc
		ret = o.appendValidDiscounts(ret, dis)
	}

	return ret, nil
}

// Discount for this order calculated using applicable discount rules
func (o *Order) CalcDiscount() (currency.Cents, error) {
	totalDiscount := currency.Cents(0)
	totalQuantity := 0

	// Get applicable discount rules
	discounts, err := o.GetDiscounts()
	if err != nil {
		return totalDiscount, err
	}

	// Calculate total quantity
	for _, li := range o.Items {
		totalQuantity += li.Quantity
	}

	// Figure out scope's price and quantity. The same scope applies to all
	// rules of a given discount.
	for _, dis := range discounts {
		price := currency.Cents(0)
		quantity := 0

		switch dis.Scope.Type {
		case scope.Product:
			// Find product this discount is scoped to
			for _, li := range o.Items {
				if li.ProductId == dis.Scope.ProductId {
					price = li.Price
					quantity = li.Quantity
					break
				}
			}
		case scope.Variant:
			// Find variant this discount is scoped to
			for _, li := range o.Items {
				if li.VariantId == dis.Scope.VariantId {
					price = li.Price
					quantity = li.Quantity
					break
				}
			}
		case scope.Collection:
			continue
		case scope.Store, scope.Organization:
			// Use total price, quantity for store and organization scopes.
			price = o.LineTotal
			quantity = totalQuantity
		}

		// Check if rule is triggered
		quantityMax := 0
		quantityIx := -1
		var priceMax currency.Cents
		priceIx := -1
		for i, rule := range dis.Rules {
			switch rule.Trigger.Type() {
			case trigger.Quantity:
				ruleQuantity := rule.Trigger.Quantity.Start
				if quantity > ruleQuantity && ruleQuantity > quantityMax {
					quantityMax = ruleQuantity
					quantityIx = i
				}
			case trigger.Price:
				rulePrice := rule.Trigger.Price.Start
				if price > rulePrice && rulePrice > priceMax {
					priceMax = rulePrice
					priceIx = i
				}
			}
		}

		// Find target
		switch dis.Target.Type {
		case target.Product:
			for _, li := range o.Items {
				if li.ProductId == dis.Target.ProductId {
					quantity = li.Quantity
					price = li.Price
					break
				}
			}
		case target.Variant:
			for _, li := range o.Items {
				if li.VariantId == dis.Target.VariantId {
					quantity = li.Quantity
					price = li.Price
					break
				}
			}
		case target.Cart:
			quantity = totalQuantity
			price = o.LineTotal
		}

		// Apply rule
		if quantityIx >= 0 {
			rule := dis.Rules[quantityIx]
			amt := rule.Action.Discount // Only handling Discount-type actions for now
			if amt.Flat != 0 {
				totalDiscount += amt.Flat
			} else if amt.Percent != 0 {
				totalDiscount += currency.Cents(float64(price) * amt.Percent)
			}
		} else if priceIx >= 0 {
			rule := dis.Rules[priceIx]
			amt := rule.Action.Discount // Only handles Discount-type actions for now
			if amt.Flat != 0 {
				totalDiscount += amt.Flat
			} else if amt.Percent != 0 {
				totalDiscount += currency.Cents(float64(price) * amt.Percent)
			}
		}
	}

	return totalDiscount, nil
}
