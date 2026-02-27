package wire

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/util/json/http"
)

type wireCreditRequest struct {
	OrderID   string  `json:"orderId"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
	Reference string  `json:"reference"`
	Notes     string  `json:"notes"`
}

type wireCreditResponse struct {
	OrderID   string `json:"orderId"`
	Status    string `json:"status"`
	Reference string `json:"reference"`
}

// Credit manually credits an account when a wire transfer is received.
// POST /api/v1/checkout/wire/credit
// Admin-only endpoint. Marks the pending wire payment as completed.
func Credit(c *gin.Context) {
	var req wireCreditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		http.Fail(c, 400, "Invalid request", err)
		return
	}

	if req.OrderID == "" {
		http.Fail(c, 400, "orderId is required", errors.New("orderId is required"))
		return
	}
	if req.Amount <= 0 {
		http.Fail(c, 400, "amount must be positive", errors.New("amount must be positive"))
		return
	}

	org := middleware.GetOrganization(c)

	// Set up the db with the namespaced context
	ctx := org.Namespaced(c)
	db := datastore.New(ctx)

	// Get the order
	ord := order.New(db)
	if err := ord.GetById(req.OrderID); err != nil {
		http.Fail(c, 404, "Order not found", err)
		return
	}

	// Mark order as paid via wire credit
	ord.Status = order.Open
	if err := ord.Put(); err != nil {
		log.Error("Failed to save order after wire credit: %v", err, c)
		http.Fail(c, 500, "Failed to update order", err)
		return
	}

	log.Info("Wire credit applied: order=%s amount=%.2f currency=%s ref=%s",
		req.OrderID, req.Amount, req.Currency, req.Reference, c)

	http.Render(c, 200, wireCreditResponse{
		OrderID:   req.OrderID,
		Status:    "credited",
		Reference: req.Reference,
	})
}
