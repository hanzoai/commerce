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

func convertTracking(t Tracking) fulfillment.Tracking {
	trk := fulfillment.Tracking{}
	trk.Number = t.Tracking
	trk.ExternalId = strconv.Itoa(t.ID)
	trk.Url = t.Url
	trk.Carrier = t.Carrier
	trk.Summary = t.Summary
	trk.FirstScanRegion = t.FirstScanRegion
	trk.FirstScanPostalCode = t.FirstScanPostalCode
	trk.FirstScanCountry = t.FirstScanCountry
	trk.DeliveryCity = t.DeliveryCity
	trk.DeliveryRegion = t.DeliveryRegion
	trk.DeliveryPostalCode = t.DeliveryPostalCode
	trk.DeliveryCountry = t.DeliveryCountry

	trk.CreatedAt = t.TrackedDate.Time
	trk.DeliveredAt = t.DeliveredDate.Time
	trk.FirstScanAt = t.FirstScanDate.Time
	trk.LabelCreatedAt = t.LabelCreatedDate.Time
	trk.SummaryAt = t.SummaryDate.Time
	return trk
}

func updateTracking(c *gin.Context, topic string, t Tracking) {
	log.Info("Tracking:\n%v", t, c)

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	ord := order.New(db)

	id := t.OrderExternalID
	err := ord.GetById(id)
	if err != nil {
		log.Warn("Unable to find order '%s': %v", id, err, c)
		c.String(200, "ok\n")
		return
	}

	ord.Fulfillment.Trackings = []fulfillment.Tracking{convertTracking(t)}

	ord.MustPut()

	c.String(200, "ok\n")
}
