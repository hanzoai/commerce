package return_

import (
	"hanzo.io/datastore"
	"hanzo.io/models/types/fulfillment"
	"hanzo.io/util/fake"
)

func Fake(db *datastore.Datastore, userId string) *Return {
	r := New(db)
	r.Fulfillment.Type = "shipwire"
	r.Fulfillment.Status = fulfillment.Pending
	r.Fulfillment.ExternalId = fake.Id()
	return r
}
