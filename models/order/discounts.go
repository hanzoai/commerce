package order

import (
	"time"

	"crowdstart.com/models/discount"
	"crowdstart.com/util/log"
)

func addDiscounts(to []*discount.Discount, from []*discount.Discount) {
	for i := 1; i < len(from); i++ {
		if from[i].Valid(time.Now()) {
			to = append(to, from[i])
		}
	}
}

func (o *Order) addOrgDiscounts(discounts []*discount.Discount) error {
	dst := make([]*discount.Discount, 0)
	_, err := discount.Query(o.Db).
		Filter("Scope=", discount.Organization).
		Filter("Enabled=", true).
		GetAll(dst)
	if err != nil {
		log.Warn("Unable to fetch discounts for organization: %v", err, o.Context())
		return err
	}
	addDiscounts(discounts, dst)
	return nil
}

func (o *Order) addStoreDiscounts(discounts []*discount.Discount) error {
	dst := make([]*discount.Discount, 0)
	_, err := discount.Query(o.Db).
		Filter("StoreId=", o.StoreId).
		Filter("Enabled=", true).
		GetAll(dst)
	if err != nil {
		log.Warn("Unable to fetch discounts for store '%s': %v", o.StoreId, err, o.Context())
		return err
	}
	addDiscounts(discounts, dst)
	return nil
}

func (o *Order) addCollectionDiscounts(discounts []*discount.Discount, id string) error {
	dst := make([]*discount.Discount, 0)
	_, err := discount.Query(o.Db).
		Filter("CollectionId=", id).
		Filter("Enabled=", true).
		GetAll(dst)
	if err != nil {
		log.Warn("Unable to fetch discounts for collection '%s': %v", id, err, o.Context())
		return err
	}

	return nil
}

func (o *Order) addProductDiscounts(discounts []*discount.Discount, id string) error {
	dst := make([]*discount.Discount, 0)
	_, err := discount.Query(o.Db).
		Filter("ProductId=", id).
		Filter("Enabled=", true).
		GetAll(dst)
	if err != nil {
		log.Warn("Unable to fetch discounts for product '%s': %v", id, err, o.Context())
		return err
	}
	addDiscounts(discounts, dst)
	return nil
}

func (o *Order) addVariantDiscounts(discounts []*discount.Discount, id string) error {
	dst := make([]*discount.Discount, 0)
	_, err := discount.Query(o.Db).
		Filter("VariantId=", id).
		Filter("Enabled=", true).
		GetAll(dst)
	if err != nil {
		log.Warn("Unable to fetch discounts for variant '%s': %v", id, err, o.Context())
	}
	addDiscounts(discounts, dst)
	return err
}

func (o *Order) GetDiscounts() ([]*discount.Discount, error) {
	discounts := make([]*discount.Discount, 0)

	// Fetch any organization-level discounts
	if err := o.addOrgDiscounts(discounts); err != nil {
		return discounts, err
	}

	// Fetch any store-level discounts
	if err := o.addStoreDiscounts(discounts); err != nil {
		return discounts, err
	}

	// Fetch any product or variant level discounts
	for _, item := range o.Items {
		if item.ProductId != "" {
			if err := o.addProductDiscounts(discounts, item.ProductId); err != nil {
				return discounts, err
			}
		} else if item.VariantId != "" {
			if err := o.addVariantDiscounts(discounts, item.VariantId); err != nil {
				return discounts, err
			}
		}
	}

	return discounts, nil
}
