package webhook

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/order"
	"hanzo.io/util/log"

	. "hanzo.io/thirdparty/shipwire/types"
)

func updateTracking(c *gin.Context, t Tracking) {
	log.Warn("Tracking Information:\n%v", t, c)

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	ord := order.New(db)
	id := t.OrderExternalId[1:]
	err := ord.GetById(id)
	if err != nil {
		log.Warn("Unable to find order '%s': %v", id, err, c)
		c.String(200, "ok\n")
		return
	}

	ord.Fulfillment.TrackingNumber = t.Tracking
	ord.Fulfillment.CreatedAt = t.LabelCreatedDate
	ord.Fulfillment.ShippedAt = t.FirstScanDate
	ord.Fulfillment.DeliveredAt = t.DeliveredDate
	ord.Fulfillment.Carrier = t.Carrier
	ord.Fulfillment.Summary = t.Summary

	// usr := user.New(db)
	// usr.MustGetById(ord.UserId)

	// pay := payment.New(db)
	// pay.MustGetById(ord.PaymentIds[0])

	// emails.SendFulfillmentEmail(db.Context, org, ord, usr, pay)
	ord.MustPut()

	// emails.SendFulfillmentEmail(db.Context, org, ord, usr, pay)

	c.String(200, "ok\n")
}
