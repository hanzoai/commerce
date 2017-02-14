package webhook

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/order"
	"hanzo.io/models/types/fulfillment"
	"hanzo.io/util/json/http"
	"hanzo.io/util/log"

	. "hanzo.io/thirdparty/shipwire/types"
)

func convertHold(h Hold) fulfillment.Hold {
	return fulfillment.Hold{
		Type:        h.Type + ":" + h.SubType,
		Description: h.Description,
		ExternalId:  h.ExternalOrderID,
		AppliedAt:   h.AppliedDate.Time,
	}
}

func updateHolds(c *gin.Context, topic string, holds []Hold) {
	log.Info("Holds:\n%v", holds, c)

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	// Handle no holds
	if len(holds) == 0 {
		c.String(200, "ok\n")
		return
	}

	// Grab first hold
	h := holds[0]

	ord := order.New(db)
	id := h.ExternalOrderID
	err := ord.GetById(id)
	if err != nil {
		log.Warn("Unable to find order '%s': %v", id, err, c)
		c.String(200, "ok\n")
		return
	}

	if err := ord.GetById(id); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	ord.Fulfillment.Status = fulfillment.Held
	ord.Fulfillment.Holds = make([]fulfillment.Hold, len(holds))
	for i := range holds {
		ord.Fulfillment.Holds[i] = convertHold(holds[i])
	}

	ord.MustPut()

	c.String(200, "ok\n")
}
