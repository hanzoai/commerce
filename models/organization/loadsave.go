package organization

import (
	"strconv"

	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/util/json"
)

func (o *Organization) Load(properties []aeds.Property) (err error) {
	o.Defaults()

	err = datastore.LoadStruct(o, properties)
	if err != nil {
		return err
	}

	if len(o.Owners_) > 0 {
		err = json.DecodeBytes([]byte(o.Owners_), &o.Owners)
	}
	return err
}

func (o *Organization) Save() ([]aeds.Property, error) {
	o.Owners_ = string(json.EncodeBytes(&o.Owners))

	props, err := datastore.SaveStruct(o)
	if err == nil {
		for k, v := range o.Owners {
			props = append(props, aeds.Property{Name: "Owners." + strconv.Itoa(k), Value: v})
		}
	}
	return props, err
}
