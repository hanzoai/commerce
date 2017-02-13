package webhook

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/order"
	"hanzo.io/models/types/fulfillment"
	"hanzo.io/util/log"

	. "hanzo.io/thirdparty/shipwire/types"
)

func updateTracking(c *gin.Context, t Tracking, isReturn bool) {
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

	tracking := fulfillment.Tracking{}
	tracking.Number = t.Tracking
	tracking.ExternalId = strconv.Itoa(t.ID)
	tracking.Url = t.Url
	tracking.Carrier = t.Carrier
	tracking.Summary = t.Summary
	tracking.FirstScanRegion = t.FirstScanRegion
	tracking.FirstScanPostalCode = t.FirstScanPostalCode
	tracking.FirstScanCountry = t.FirstScanCountry
	tracking.DeliveryCity = t.DeliveryCity
	tracking.DeliveryRegion = t.DeliveryRegion
	tracking.DeliveryPostalCode = t.DeliveryPostalCode
	tracking.DeliveryCountry = t.DeliveryCountry

	tracking.CreatedAt = t.TrackedDate.Time
	tracking.DeliveredAt = t.DeliveredDate.Time
	tracking.FirstScanAt = t.FirstScanDate.Time
	tracking.LabelCreatedAt = t.LabelCreatedDate.Time
	tracking.SummaryAt = t.SummaryDate.Time

	if isReturn {
		ord.Fulfillment.Return.Tracking = tracking
	} else {
		ord.Fulfillment.Tracking = tracking
	}

	ord.MustPut()

	c.String(200, "ok\n")
}
