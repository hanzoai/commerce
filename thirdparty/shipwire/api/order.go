package api

import (
	"fmt"
	"strconv"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/types/fulfillment"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/thirdparty/shipwire"
	"github.com/hanzoai/commerce/util/counter"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"

	. "github.com/hanzoai/commerce/thirdparty/shipwire/types"
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

func updateOrder(c *zip.Ctx, topic string, o Order) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

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

	oldPricing := ord.Fulfillment.Pricing

	// Save Shipwire data
	ord.Fulfillment.Type = fulfillment.Shipwire
	ord.Fulfillment.ExternalId = strconv.Itoa(o.ID)

	// Update fulfillment states
	ord.Fulfillment.Status = fulfillment.Status(o.Status)
	// Shipwire prices in USD, so the USD scale is the exact conversion. A total
	// we cannot read leaves the old pricing standing rather than overwriting a
	// real cost with a confident zero.
	if cents, err := currency.USD.Parse(o.Pricing.Resource.Total.String()); err == nil {
		ord.Fulfillment.Pricing = cents
	} else {
		log.Warn("Unable to read shipwire pricing for order '%s': %v", id, err, c)
	}
	if cents, err := currency.USD.Parse(o.PricingEstimate.Resource.Total.String()); err == nil {
		ord.Fulfillment.PricingEstimate = cents
	} else {
		log.Warn("Unable to read shipwire pricing estimate for order '%s': %v", id, err, c)
	}
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

	if oldPricing != ord.Fulfillment.Pricing && ord.Fulfillment.Pricing != 0 {
		counter.IncrOrderShip(db.Context, ord, time.Now())
	}
}

func createOrder(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	// Decode return options
	opts := OrderOptions{}
	if err := json.Unmarshal(c.Body(), &opts); err != nil {
		return http.Fail(c, 400, fmt.Errorf("Failed to decode request body: %v", err), err)
	}

	// Fetch order
	id := c.Param("orderid")
	ord := order.New(db)
	if err := ord.GetById(id); err != nil {
		return http.Fail(c, 404, fmt.Errorf("Unable to find order '%s'", id), err)
	}

	// Fetch user
	usr := user.New(db)
	if err := usr.GetById(ord.UserId); err != nil {
		return http.Fail(c, 404, fmt.Errorf("Unable to find user '%s'", ord.UserId), err)
	}

	// Create order in Shipwire
	client := shipwire.New(c, org.Shipwire.Username, org.Shipwire.Password)
	_, res, err := client.CreateOrder(ord, usr, opts)

	if err != nil {
		return http.Fail(c, res.Status, res.Message, err)
	}

	return http.Render(c, 200, ord)
}
