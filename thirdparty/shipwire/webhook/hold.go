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

func updateHolds(c *gin.Context, holds []Hold) {
	log.Info("Holds:\n%v", holds, c)

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

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
		ord.Fulfillment.Holds[i] = fulfillment.Hold{
			Type:        holds[i].Type + ":" + holds[i].SubType,
			Description: holds[i].Description,
			ExternalId:  holds[i].ExternalOrderID,
			AppliedAt:   holds[i].AppliedDate.Time,
		}
	}

	// ord.Fulfillment.TrackingNumber = t.Tracking
	// ord.Fulfillment.CreatedAt = t.LabelCreatedDate
	// ord.Fulfillment.ShippedAt = t.FirstScanDate
	// ord.Fulfillment.DeliveredAt = t.DeliveredDate
	// // ord.Fulfillment.Service = req.Service
	// ord.Fulfillment.Carrier = t.Carrier
	// ord.Fulfillment.Carrier = t.Summary

	// usr := user.New(db)
	// usr.MustGetById(ord.UserId)

	// pay := payment.New(db)
	// pay.MustGetById(ord.PaymentIds[0])

	// emails.SendFulfillmentEmail(db.Context, org, ord, usr, pay)
	ord.MustPut()

	// emails.SendFulfillmentEmail(db.Context, org, ord, usr, pay)

	c.String(200, "ok\n")
}
