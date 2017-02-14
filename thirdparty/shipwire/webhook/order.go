package webhook

import (
	"github.com/gin-gonic/gin"
	"strconv"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/order"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/types/fulfillment"
	"hanzo.io/util/json"
	"hanzo.io/util/log"

	. "hanzo.io/thirdparty/shipwire/types"
)

func updateFromTrackings(ord *order.Order, rsrc Resource) {
	if len(rsrc.Items) < 1 {
		return
	}

	trackings := make([]fulfillment.Tracking, len(rsrc.Items))
	for i, item := range rsrc.Items {
		var t Tracking
		if err := json.Unmarshal(item.Resource, &t); err == nil {
			trackings[i] = convertTracking(t)
		}
	}
	ord.Fulfillment.Trackings = trackings
}

func updateFromHolds(ord *order.Order, rsrc Resource) {
	if len(rsrc.Items) < 1 {
		return
	}

	holds := make([]fulfillment.Hold, len(rsrc.Items))
	for i, item := range rsrc.Items {
		var h Hold
		if err := json.Unmarshal(item.Resource, &h); err == nil {
			holds[i] = convertHold(h)
		}
	}
	ord.Fulfillment.Holds = holds
}

func updateOrder(c *gin.Context, topic string, o Order) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	ord := order.New(db)
	id := o.ExternalID
	if id == "" {
		id = o.OrderNo
	}

	log.Info("Updating order '%s'", id, c)
	err := ord.GetById(id)
	if err != nil {
		log.Warn("Unable to find order '%s': %v", id, err, c)
		return
	}

	// Save Shipwire data
	ord.Fulfillment.Type = fulfillment.Shipwire
	ord.Fulfillment.ExternalId = strconv.Itoa(o.ID)

	// Update fulfillment states
	ord.FulfillmentStatus = fulfillment.Status(o.Status)
	ord.Fulfillment.Status = fulfillment.Status(o.Status)
	ord.Fulfillment.Pricing = currency.Cents(o.Pricing.Resource.Total * 100)
	ord.Fulfillment.PricingEstimate = currency.Cents(o.PricingEstimate.Resource.Total * 100)
	ord.Fulfillment.SameDay = o.Options.Resource.SameDay
	ord.Fulfillment.Service = o.Options.Resource.ServiceLevelCode
	ord.Fulfillment.Carrier = o.Options.Resource.CarrierCode
	ord.Fulfillment.WarehouseId = strconv.Itoa(o.Options.Resource.WarehouseID)
	ord.Fulfillment.WarehouseRegion = o.Options.Resource.WarehouseRegion

	// Update dates
	ord.Fulfillment.CreatedAt = o.Events.Resource.CreatedDate.Time
	ord.Fulfillment.CancelledAt = o.Events.Resource.CancelledDate.Time
	ord.Fulfillment.CompletedAt = o.Events.Resource.CompletedDate.Time
	ord.Fulfillment.CreatedAt = o.Events.Resource.CreatedDate.Time
	ord.Fulfillment.ExpectedCompletedAt = o.Events.Resource.ExpectedCompletedDate.Time
	ord.Fulfillment.ExpectedAt = o.Events.Resource.ExpectedDate.Time
	ord.Fulfillment.ExpectedSubmittedAt = o.Events.Resource.ExpectedSubmittedDate.Time
	ord.Fulfillment.LastManualUpdateAt = o.Events.Resource.LastManualUpdateDate.Time
	ord.Fulfillment.PickedUpAt = o.Events.Resource.PickedUpDate.Time
	ord.Fulfillment.ProcessedAt = o.Events.Resource.ProcessedDate.Time
	ord.Fulfillment.ReturnedAt = o.Events.Resource.ReturnedDate.Time
	ord.Fulfillment.SubmittedAt = o.Events.Resource.SubmittedDate.Time

	updateFromTrackings(ord, o.Trackings.Resource)
	updateFromHolds(ord, o.Holds.Resource)

	ord.MustPut()

	c.String(200, "ok\n")
}
