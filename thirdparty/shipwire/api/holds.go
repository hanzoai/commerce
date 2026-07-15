package api

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/types/fulfillment"

	. "github.com/hanzoai/commerce/thirdparty/shipwire/types"
)

func convertHold(h Hold) fulfillment.Hold {
	return fulfillment.Hold{
		Type:        h.Type + ":" + h.SubType,
		Description: h.Description,
		ExternalId:  h.ExternalOrderID,
		AppliedAt:   h.AppliedDate.Time,
	}
}

func updateHolds(c *zip.Ctx, topic string, holds []Hold) {
	log.Info("Holds:\n%v", holds, c)

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	// Handle no holds
	if len(holds) == 0 {
		return
	}

	// Grab first hold
	h := holds[0]

	ord := order.New(db)
	id := h.ExternalOrderID
	err := ord.GetById(id)
	if err != nil {
		log.Warn("Unable to find order '%s': %v", id, err, c)
		return
	}

	if err := ord.GetById(id); err != nil {
		log.Error("Failed decode request body: %v", err, c)
		return
	}

	ord.Fulfillment.Status = fulfillment.Held
	ord.Fulfillment.Holds = make([]fulfillment.Hold, len(holds))
	for i := range holds {
		ord.Fulfillment.Holds[i] = convertHold(holds[i])
	}

	ord.MustPut()
}
