package inventory

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	inventoryModel "github.com/hanzoai/commerce/models/inventory"
	"github.com/hanzoai/commerce/models/inventorylevel"
	"github.com/hanzoai/commerce/models/reservation"
	"github.com/hanzoai/commerce/models/variantinventorylink"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rest"
)

func Route(router zip.Router, args ...zip.Handler) {
	namespaced := middleware.Namespace()

	// Inventory Items - standard CRUD
	rest.New(inventoryModel.InventoryItem{}).Route(router, args...)

	// Inventory Levels with stock adjustment endpoint
	levelApi := rest.New(inventorylevel.InventoryLevel{})
	levelApi.POST("/:inventorylevelid/adjust", namespaced, AdjustStock)
	levelApi.Route(router, args...)

	// Reservations
	rest.New(reservation.ReservationItem{}).Route(router, args...)

	// Variant-Inventory Links
	rest.New(variantinventorylink.VariantInventoryLink{}).Route(router, args...)
}

// adjustRequest represents a stock adjustment request body.
type adjustRequest struct {
	StockedQuantity  *int `json:"stockedQuantity"`
	ReservedQuantity *int `json:"reservedQuantity"`
	IncomingQuantity *int `json:"incomingQuantity"`
}

// AdjustStock adjusts StockedQuantity, ReservedQuantity, and/or IncomingQuantity
// on an InventoryLevel by the delta values provided in the request body.
func AdjustStock(c *zip.Ctx) error {
	ctx := c.Context()
	db := datastore.New(ctx)
	id := c.Param("inventorylevelid")

	// Load existing inventory level
	level := inventorylevel.New(db)
	if err := level.GetById(id); err != nil {
		return http.Fail(c, 404, "Inventory level not found", err)
	}

	// Parse adjustment
	var adj adjustRequest
	if err := json.DecodeBytes(c.Body(), &adj); err != nil {
		return http.Fail(c, 400, "Failed to decode request body", err)
	}

	// Compute the post-adjustment values WITHOUT mutating yet, so a rejected
	// oversell leaves the stored level untouched.
	stocked := level.StockedQuantity
	reserved := level.ReservedQuantity
	incoming := level.IncomingQuantity
	if adj.StockedQuantity != nil {
		stocked += *adj.StockedQuantity
	}
	if adj.ReservedQuantity != nil {
		reserved += *adj.ReservedQuantity
	}
	if adj.IncomingQuantity != nil {
		incoming += *adj.IncomingQuantity
	}

	// Oversell guard: available (stocked − reserved) must never go negative.
	// Reserving more than is on hand — or stocking down below what is already
	// reserved — would sell inventory that does not exist. Refuse and DO NOT
	// persist. Incoming/restock (positive stocked) and releasing a reservation
	// (negative reserved) stay allowed because they keep available ≥ 0.
	if stocked-reserved < 0 {
		return http.Fail(c, 409, "insufficient available stock", nil)
	}

	level.StockedQuantity = stocked
	level.ReservedQuantity = reserved
	level.IncomingQuantity = incoming

	// Persist
	if err := level.Update(); err != nil {
		return http.Fail(c, 500, "Failed to adjust inventory level", err)
	}

	return http.Render(c, 200, level)
}
