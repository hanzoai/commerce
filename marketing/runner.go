package marketing

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/marketing/engines"
	"github.com/hanzoai/commerce/marketing/types"
	"github.com/hanzoai/commerce/models/ads/adcampaign"
	"github.com/hanzoai/commerce/models/multi"
)

func Create(db *datastore.Datastore, ci types.CreateInput) (cmpgn *adcampaign.AdCampaign, err error) {
	co := types.CreateOutput{
		nil,
		[]interface{}{},
	}

	switch ci.Engine {
	default:
		if co, err = (engines.DemoEngine{}.Create(db, ci)); err != nil {
			return nil, err
		}
	}

	return co.AdCampaign, multi.Create(co.Entities)
}
