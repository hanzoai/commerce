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

type usageRequest struct {
	User             string `json:"user"`
	Currency         string `json:"currency"`
	Amount           int64  `json:"amount"` // cents
	Model            string `json:"model"`
	Provider         string `json:"provider"`
	PromptTokens     int    `json:"promptTokens"`
	CompletionTokens int    `json:"completionTokens"`
	TotalTokens      int    `json:"totalTokens"`
	RequestID        string `json:"requestId"`
	Premium          bool   `json:"premium"`
	Stream           bool   `json:"stream"`
	Status           string `json:"status"`
	ClientIP         string `json:"clientIp"`
}

// GetUsage returns usage transactions for an IAM user, filtered by tag "api-usage".
//
//	GET /api/v1/billing/usage?user=hanzo/alice&currency=usd
func GetUsage(c *gin.Context) {
	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c)
	db := datastore.New(ctx)

	user := strings.TrimSpace(c.Query("user"))
	if user == "" {
		http.Fail(c, 400, "user query parameter is required", nil)
		return
	}

	rootKey := db.NewKey("synckey", "", 1, nil)

	transs := make([]*transaction.Transaction, 0)
	q := transaction.Query(db).Ancestor(rootKey).
		Filter("Test=", !org.Live).
		Filter("SourceKind=", "iam-user").
		Filter("SourceId=", user).
		Filter("Tags=", "api-usage")

	cur := currency.Type(strings.ToLower(c.DefaultQuery("currency", "")))
	if cur != "" {
		q = q.Filter("Currency=", cur)
	}

	if _, err := q.GetAll(&transs); err != nil {
		log.Error("Failed to query usage transactions: %v", err, c)
		http.Fail(c, 500, "failed to query usage", err)
		return
	}

	items := make([]gin.H, 0, len(transs))
	for _, t := range transs {
		items = append(items, gin.H{
			"transactionId": t.Id(),
			"amount":        t.Amount,
			"currency":      t.Currency,
			"notes":         t.Notes,
			"metadata":      t.Metadata,
			"createdAt":     t.CreatedAt,
		})
	}

	c.JSON(200, gin.H{
		"user":  user,
		"count": len(items),
		"usage": items,
	})
}

// RecordUsage records an API usage event as a Withdraw transaction.
//
//	POST /api/v1/billing/usage
//
// Creates a withdraw transaction deducting the cost from the user's balance.
func RecordUsage(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	var req usageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		http.Fail(c, 400, "invalid request body", err)
		return
	}

	if req.User == "" {
		http.Fail(c, 400, "user is required", nil)
		return
	}

	if req.Amount <= 0 {
		// Zero-cost usage — just acknowledge, no transaction needed
		c.JSON(200, gin.H{
			"user":   req.User,
			"amount": 0,
			"status": "skipped",
		})
		return
	}

	cur := currency.Type(strings.ToLower(req.Currency))
	if cur == "" {
		cur = "usd"
	}

	notes := fmt.Sprintf("API usage: %s (%d tokens)", req.Model, req.TotalTokens)

	trans := transaction.New(db)
	trans.Type = transaction.Withdraw
	trans.SourceId = req.User
	trans.SourceKind = "iam-user"
	trans.Currency = cur
	trans.Amount = currency.Cents(req.Amount)
	trans.Notes = notes
	trans.Tags = "api-usage"
	trans.Metadata = Map{
		"model":            req.Model,
		"provider":         req.Provider,
		"promptTokens":     req.PromptTokens,
		"completionTokens": req.CompletionTokens,
		"totalTokens":      req.TotalTokens,
		"requestId":        req.RequestID,
		"premium":          req.Premium,
		"stream":           req.Stream,
		"status":           req.Status,
		"clientIp":         req.ClientIP,
	}

	if !org.Live {
		trans.Test = true
	}

	// Create the transaction. For usage recording we do NOT enforce
	// balance checks — the API call already happened and must be recorded.
	// Balance gating happens at request time in Cloud-API.
	if err := trans.Create(); err != nil {
		log.Error("Failed to record usage transaction: %v", err, c)
		http.Fail(c, 500, "failed to record usage", err)
		return
	}

	c.JSON(201, gin.H{
		"transactionId": trans.Id(),
		"user":          req.User,
		"amount":        req.Amount,
		"currency":      cur,
		"type":          "withdraw",
	})
}
