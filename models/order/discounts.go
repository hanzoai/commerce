package order

import (
	"time"

	"crowdstart.com/models/discount"
	"crowdstart.com/util/log"
)

type discountChan chan []*discount.Discount
type errorChan chan error

func addDiscounts(to []*discount.Discount, from []*discount.Discount) {
	now := time.Now()
	for i := 0; i < len(from); i++ {
		if from[i].Valid(now) {
			to = append(to, from[i])
		}
	}
}

func (o *Order) addOrgDiscounts(disc discountChan, errc errorChan) {
	dst := make([]*discount.Discount, 0)
	_, err := discount.Query(o.Db).
		Filter("Scope=", discount.Organization).
		Filter("Enabled=", true).
		GetAll(dst)
	if err != nil {
		log.Warn("Unable to fetch discounts for organization: %v", err, o.Context())
	}
	errc <- err
	disc <- dst
}

func (o *Order) addStoreDiscounts(disc discountChan, errc errorChan) {
	dst := make([]*discount.Discount, 0)
	_, err := discount.Query(o.Db).
		Filter("StoreId=", o.StoreId).
		Filter("Enabled=", true).
		GetAll(dst)
	if err != nil {
		log.Warn("Unable to fetch discounts for store '%s': %v", o.StoreId, err, o.Context())
	}
	errc <- err
	disc <- dst
}

func (o *Order) addCollectionDiscounts(id string, disc discountChan, errc errorChan) {
	dst := make([]*discount.Discount, 0)
	_, err := discount.Query(o.Db).
		Filter("CollectionId=", id).
		Filter("Enabled=", true).
		GetAll(dst)
	if err != nil {
		log.Warn("Unable to fetch discounts for collection '%s': %v", id, err, o.Context())
	}
	errc <- err
	disc <- dst
}

func (o *Order) addProductDiscounts(id string, disc discountChan, errc errorChan) {
	dst := make([]*discount.Discount, 0)
	_, err := discount.Query(o.Db).
		Filter("ProductId=", id).
		Filter("Enabled=", true).
		GetAll(dst)
	if err != nil {
		log.Warn("Unable to fetch discounts for product '%s': %v", id, err, o.Context())
	}
	errc <- err
	disc <- dst
}

func (o *Order) addVariantDiscounts(id string, disc discountChan, errc errorChan) {
	dst := make([]*discount.Discount, 0)
	_, err := discount.Query(o.Db).
		Filter("VariantId=", id).
		Filter("Enabled=", true).
		GetAll(dst)
	if err != nil {
		log.Warn("Unable to fetch discounts for variant '%s': %v", id, err, o.Context())
	}
	errc <- err
	disc <- dst
}

func (o *Order) GetDiscounts() ([]*discount.Discount, error) {
	discounts := make([]*discount.Discount, 0)

	channels := 2 + len(o.Items)
	errc := make(chan error, channels)
	disc := make(chan []*discount.Discount, channels)

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
	for err := range errc {
		if err != nil {
			return nil, err
		}
	}

	// Merge results together
	for dis := range discounts {
		addDiscounts(discounts, dis)
	}

	return discounts, err
}
