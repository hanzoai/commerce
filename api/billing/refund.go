package billing

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"

	. "github.com/hanzoai/commerce/types"
)

type refundRequest struct {
	User                  string `json:"user"`
	Currency              string `json:"currency"`
	Amount                int64  `json:"amount"` // cents
	OriginalTransactionID string `json:"originalTransactionId"`
	Notes                 string `json:"notes"`
}

// Refund creates a deposit tagged "refund" to correct an overcharge.
// The metadata links back to the original transaction for auditability.
//
//	POST /api/v1/billing/refund
func Refund(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	var req refundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		http.Fail(c, 400, "invalid request body", err)
		return
	}

	if req.User == "" {
		http.Fail(c, 400, "user is required", nil)
		return
	}

	if req.Amount <= 0 {
		http.Fail(c, 400, "amount must be positive", nil)
		return
	}

	if req.OriginalTransactionID == "" {
		http.Fail(c, 400, "originalTransactionId is required", nil)
		return
	}

	cur := currency.Type(strings.ToLower(req.Currency))
	if cur == "" {
		cur = "usd"
	}

	notes := req.Notes
	if notes == "" {
		notes = fmt.Sprintf("Refund: %d cents %s (original tx: %s)", req.Amount, cur, req.OriginalTransactionID)
	}

	trans := transaction.New(db)
	trans.Type = transaction.Deposit
	trans.DestinationId = req.User
	trans.DestinationKind = "iam-user"
	trans.Currency = cur
	trans.Amount = currency.Cents(req.Amount)
	trans.Notes = notes
	trans.Tags = "refund"
	trans.Metadata = Map{
		"refundType":            "billing-correction",
		"originalTransactionId": req.OriginalTransactionID,
	}

	if !org.Live {
		trans.Test = true
	}

	if err := trans.Create(); err != nil {
		log.Error("Failed to create refund transaction: %v", err, c)
		http.Fail(c, 500, "failed to create refund", err)
		return
	}

	c.JSON(201, gin.H{
		"transactionId":         trans.Id(),
		"user":                  req.User,
		"amount":                req.Amount,
		"currency":              cur,
		"type":                  "deposit",
		"tags":                  "refund",
		"originalTransactionId": req.OriginalTransactionID,
	})
}
