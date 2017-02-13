package webhook

import (
	"strconv"

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
	id := t.OrderExternalID[1:]
	err := ord.GetById(id)
	if err != nil {
		log.Warn("Unable to find order '%s': %v", id, err, c)
		c.String(200, "ok\n")
		return
	}

	ord.Fulfillment.Tracking.Number = t.Tracking
	ord.Fulfillment.Tracking.ExternalId = strconv.Itoa(t.ID)
	ord.Fulfillment.Tracking.Url = t.Url
	ord.Fulfillment.Tracking.CreatedAt = t.TrackedDate
	ord.Fulfillment.Tracking.Carrier = t.Carrier
	ord.Fulfillment.Tracking.Summary = t.Summary
	ord.Fulfillment.Tracking.SummaryAt = t.SummaryDate
	ord.Fulfillment.Tracking.LabelCreatedAt = t.LabelCreatedDate
	ord.Fulfillment.Tracking.FirstScanRegion = t.FirstScanRegion
	ord.Fulfillment.Tracking.FirstScanPostalCode = t.FirstScanPostalCode
	ord.Fulfillment.Tracking.FirstScanCountry = t.FirstScanCountry
	ord.Fulfillment.Tracking.FirstScanAt = t.FirstScanDate
	ord.Fulfillment.Tracking.DeliveryCity = t.DeliveryCity
	ord.Fulfillment.Tracking.DeliveryRegion = t.DeliveryRegion
	ord.Fulfillment.Tracking.DeliveryPostalCode = t.DeliveryPostalCode
	ord.Fulfillment.Tracking.DeliveryCountry = t.DeliveryCountry
	ord.Fulfillment.Tracking.DeliveredAt = t.DeliveredDate

	ord.MustPut()

	c.String(200, "ok\n")
}
