package bundle

import (
	"strconv"

	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/util/json"
)

func (b *Bundle) Load(properties []aeds.Property) (err error) {
	b.Defaults()

	err = datastore.LoadStruct(b, properties)
	if err != nil {
		return err
	}

	if len(b.Media_) > 0 {
		err = json.DecodeBytes([]byte(b.Media_), &b.Media)
	}

	if err != nil {
		return err
	}

	if len(b.ProductIds_) > 0 {
		err = json.DecodeBytes([]byte(b.ProductIds_), &b.ProductIds)
	}

	if err != nil {
		return err
	}

	if len(b.VariantIds_) > 0 {
		err = json.DecodeBytes([]byte(b.VariantIds_), &b.VariantIds)
	}
	return err
}

func (b *Bundle) Save() ([]aeds.Property, error) {
	b.Media_ = string(json.EncodeBytes(&b.Media))
	b.ProductIds_ = string(json.EncodeBytes(&b.ProductIds))
	b.VariantIds_ = string(json.EncodeBytes(&b.VariantIds))

	props, err := datastore.SaveStruct(b)
	if err == nil {
		for k, v := range b.ProductIds {
			props = append(props, aeds.Property{Name: "ProductIds." + strconv.Itoa(k), Value: v})
		}
		for k, v := range b.VariantIds {
			props = append(props, aeds.Property{Name: "VariantIds." + strconv.Itoa(k), Value: v})
		}
	}
	return props, err
}
