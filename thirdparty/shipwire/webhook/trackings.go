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

func updateTrackings(c *gin.Context, trackings []Tracking) {
	log.Warn("Trackings:\n%v", trackings, c)

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	ord := order.New(db)

	t := trackings[0]
	id := t.OrderExternalID
	err := ord.GetById(id)
	if err != nil {
		log.Warn("Unable to find order '%s': %v", id, err, c)
		c.String(200, "ok\n")
		return
	}

	ord.Fulfillment.Trackings = make([]fulfillment.Tracking, 0)
	for _, t := range trackings {
		ord.Fulfillment.Trackings = append(ord.Fulfillment.Trackings, convertTracking(t))
	}

	ord.MustPut()

	c.String(200, "ok\n")
}
