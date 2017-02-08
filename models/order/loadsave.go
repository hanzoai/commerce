package order

import (
	"strconv"

	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/util/json"
)

func (o *Order) Load(properties []aeds.Property) (err error) {
	// Ensure we're initialized
	o.Defaults()

	// Load supported properties
	err = datastore.LoadStruct(o, properties)
	if err != nil {
		return err
	}

	// Set order number
	o.Number = o.NumberFromId()

	// Deserialize from datastore
	if len(o.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(o.Metadata_), &o.Metadata)
	}

	if err != nil {
		return err
	}
	if len(o.Items_) > 0 {
		err = json.DecodeBytes([]byte(o.Items_), &o.Items)
	}
	if err != nil {
		return err
	}
	if len(o.CouponCodes_) > 0 {
		err = json.DecodeBytes([]byte(o.CouponCodes_), &o.CouponCodes)
	}
	if err != nil {
		return err
	}
	if len(o.PaymentIds_) > 0 {
		err = json.DecodeBytes([]byte(o.PaymentIds_), &o.PaymentIds)
	}
	if err != nil {
		return err
	}
	return err
}

func (o *Order) Save() ([]aeds.Property, error) {
	// Serialize unsupported properties
	o.Metadata_ = string(json.EncodeBytes(&o.Metadata))
	o.CouponCodes_ = string(json.EncodeBytes(&o.CouponCodes))
	o.Items_ = string(json.EncodeBytes(o.Items))
	o.PaymentIds_ = string(json.EncodeBytes(o.PaymentIds))

	props, err := datastore.SaveStruct(o)

	if err == nil {
		for k, v := range o.CouponCodes {
			props = append(props, aeds.Property{Name: "CouponCodes." + strconv.Itoa(k), Value: v})
		}
		for k, li := range o.Items {
			itemsPrefix := "Items." + strconv.Itoa(k) + "."
			vals := li.ToMap()
			for k, v := range vals {
				props = append(props, aeds.Property{Name: itemsPrefix + string(k), Value: v})
			}
		}
		for k, v := range o.PaymentIds {
			props = append(props, aeds.Property{Name: "PaymentIds." + strconv.Itoa(k), Value: v})
		}
	}
	return props, err
}
