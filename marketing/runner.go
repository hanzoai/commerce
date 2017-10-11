package marketing

import (
	"hanzo.io/datastore"
	"hanzo.io/marketing/engines"
	"hanzo.io/marketing/types"
	"hanzo.io/models/ads/adcampaign"
	"hanzo.io/models/multi"
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
