package fulfillment

import (
	"errors"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/fulfillmentmodel"
	"github.com/hanzoai/commerce/models/fulfillmentprovider"
	"github.com/hanzoai/commerce/models/fulfillmentset"
	"github.com/hanzoai/commerce/models/geozone"
	"github.com/hanzoai/commerce/models/servicezone"
	"github.com/hanzoai/commerce/models/shippingoption"
	"github.com/hanzoai/commerce/models/shippingoptionrule"
	"github.com/hanzoai/commerce/models/shippingprofile"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rest"
)

func Route(router zip.Router, args ...zip.Handler) {
	namespaced := middleware.Namespace()

	rest.New(fulfillmentset.FulfillmentSet{}).Route(router, args...)
	rest.New(servicezone.ServiceZone{}).Route(router, args...)
	rest.New(geozone.GeoZone{}).Route(router, args...)
	rest.New(shippingoption.ShippingOption{}).Route(router, args...)
	rest.New(shippingoptionrule.ShippingOptionRule{}).Route(router, args...)
	rest.New(shippingprofile.ShippingProfile{}).Route(router, args...)
	rest.New(fulfillmentprovider.FulfillmentProvider{}).Route(router, args...)

	fApi := rest.New(fulfillmentmodel.Fulfillment{})
	fApi.POST("/:fulfillmentid/ship", namespaced, Ship)
	fApi.POST("/:fulfillmentid/cancel", namespaced, Cancel)
	fApi.Route(router, args...)
}

// ShipRequest holds the optional tracking labels sent when marking a
// fulfillment as shipped.
type ShipRequest struct {
	Labels []fulfillmentmodel.FulfillmentLabel `json:"labels"`
}

// Ship marks a fulfillment as shipped by setting ShippedAt to now and
// optionally appending tracking labels provided in the request body.
func Ship(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	id := c.Param("fulfillmentid")

	f := fulfillmentmodel.New(db)
	if err := f.GetById(id); err != nil {
		return http.Fail(c, 404, "No fulfillment found with id: "+id, err)
	}

	if f.CanceledAt != nil {
		return http.Fail(c, 400, "Fulfillment has been canceled", errors.New("fulfillment canceled"))
	}

	// Parse optional labels from request body
	req := ShipRequest{}
	if len(c.Body()) > 0 {
		if err := json.DecodeBytes(c.Body(), &req); err != nil {
			return http.Fail(c, 400, "Failed to decode request body", err)
		}
	}

	now := time.Now()
	f.ShippedAt = &now

	if len(req.Labels) > 0 {
		f.Labels = append(f.Labels, req.Labels...)
	}

	if err := f.Update(); err != nil {
		return http.Fail(c, 500, "Failed to update fulfillment", err)
	}

	return http.Render(c, 200, f)
}

// Cancel marks a fulfillment as canceled by setting CanceledAt to now.
func Cancel(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	id := c.Param("fulfillmentid")

	f := fulfillmentmodel.New(db)
	if err := f.GetById(id); err != nil {
		return http.Fail(c, 404, "No fulfillment found with id: "+id, err)
	}

	if f.CanceledAt != nil {
		return http.Fail(c, 400, "Fulfillment is already canceled", errors.New("already canceled"))
	}

	now := time.Now()
	f.CanceledAt = &now

	if err := f.Update(); err != nil {
		return http.Fail(c, 500, "Failed to cancel fulfillment", err)
	}

	return http.Render(c, 200, f)
}
