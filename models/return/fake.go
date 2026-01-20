package return_

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/types/fulfillment"
	"github.com/hanzoai/commerce/util/fake"
)

func Fake(db *datastore.Datastore, userId string) *Return {
	r := New(db)
	r.Fulfillment.Type = "shipwire"
	r.Fulfillment.Status = fulfillment.Pending
	r.Fulfillment.ExternalId = fake.Id()
	return r
}
