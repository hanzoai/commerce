package collection

import (
	"strconv"

	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/util/json"
)

func (c *Collection) Load(properties []aeds.Property) (err error) {
	c.Defaults()

	err = datastore.LoadStruct(c, properties)
	if err != nil {
		return err
	}

	if len(c.Media_) > 0 {
		err = json.DecodeBytes([]byte(c.Media_), &c.Media)
	}

	if err != nil {
		return err
	}

	if len(c.ProductIds_) > 0 {
		err = json.DecodeBytes([]byte(c.ProductIds_), &c.ProductIds)
	}

	if err != nil {
		return err
	}

	if len(c.VariantIds_) > 0 {
		err = json.DecodeBytes([]byte(c.VariantIds_), &c.VariantIds)
	}
	return err
}

func (c *Collection) Save() ([]aeds.Property, error) {
	c.Media_ = string(json.EncodeBytes(&c.Media))
	c.ProductIds_ = string(json.EncodeBytes(&c.ProductIds))
	c.VariantIds_ = string(json.EncodeBytes(&c.VariantIds))

	props, err := datastore.SaveStruct(c)
	if err == nil {
		for k, v := range c.ProductIds {
			props = append(props, aeds.Property{Name: "ProductIds." + strconv.Itoa(k), Value: v})
		}
		for k, v := range c.VariantIds {
			props = append(props, aeds.Property{Name: "VariantIds." + strconv.Itoa(k), Value: v})
		}
	}
	return props, err
}
