package migrations

import (
	"encoding/gob"
	"errors"

	"appengine"
	"appengine/datastore"

	. "crowdstart.io/models"
	"crowdstart.io/util/parallel"
)

type orderIdFixer struct {
	SomeString string
}

func (f orderIdFixer) NewObject() interface{} {
	return new(Order)
}

func (f orderIdFixer) Execute(c appengine.Context, key *datastore.Key, object interface{}) error {
	var ok bool
	var o *Order
	if o, ok = object.(*Order); !ok {
		return errors.New("Object should be of type 'order'")
	}

	o.Id = key.Encode()
	if _, err := datastore.Put(c, key, o); err != nil {
		return err
	}

	return nil
}

// Gob registration
func init() {
	gob.Register(orderIdFixer{})
}

func fixOrderIds(c appengine.Context) {
	parallel.DatastoreJob(c, "order", 30, orderIdFixer{})
}
