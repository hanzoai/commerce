package migrations

import (
	"errors"

	"appengine"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
)

var addEmailToContribution(c appengine.Context) {
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

func addEmailToContribution(c appengine.Context) {
	parallel.DatastoreJob(c, "order", 50, orderIdFixer{})
}
