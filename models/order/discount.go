package order

import (
	"crowdstart.com/models/discount"
	"crowdstart.com/models/discount/scope"
	"crowdstart.com/models/discount/target"
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
	discounts, err := o.GetDiscounts()
	var discountTotal currency.Cents
	if err != nil {
		return discountTotal, err
	}
	totalQuantity := 0
	for _, li := range o.Items {
		totalQuantity += li.Quantity
	}
	for _, dis := range discounts {
		quantity := 0
		var price currency.Cents
		switch dis.Scope.Type {
		case scope.Product:
			for _, li := range o.Items {
				if li.ProductId == dis.Scope.ProductId {
					quantity = li.Quantity
					price = li.Price
					break
				}
			}
		case scope.Variant:
			for _, li := range o.Items {
				if li.VariantId == dis.Scope.VariantId {
					quantity = li.Quantity
					price = li.Price
					break
				}
			}
		case scope.Collection:
			continue
		case scope.Store:
			quantity = totalQuantity
			price = o.LineTotal
		}

		quantityMax := 0
		quantityIx := -1
		var priceMax currency.Cents
		priceIx := -1
		for i, rule := range dis.Rules {
			ruleQuantity := rule.Range.Quantity.Start
			if ruleQuantity != 0 {
				if quantity > ruleQuantity && ruleQuantity > quantityMax {
					quantityMax = ruleQuantity
					quantityIx = i
					continue
				}
			}
			rulePrice := rule.Range.Price.Start
			if rulePrice != 0 {
				if price > rulePrice && rulePrice > priceMax {
					priceMax = rulePrice
					priceIx = i
					continue
				}
			}
		}

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

		if quantityIx >= 0 {
			rule := dis.Rules[quantityIx]
			if rule.Amount.Flat != 0 {
				discountTotal += currency.Cents(rule.Amount.Flat)
			} else if rule.Amount.Percent != 0 {
				discountTotal += currency.Cents(float64(price) * rule.Amount.Percent)
			}
		} else if priceIx >= 0 {
			rule := dis.Rules[priceIx]
			if rule.Amount.Flat != 0 {
				discountTotal += currency.Cents(rule.Amount.Flat)
			} else if rule.Amount.Percent != 0 {
				discountTotal += currency.Cents(float64(price) * rule.Amount.Percent)
			}
		}
	}
	return discountTotal, nil
}
