package billing

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"

	. "github.com/hanzoai/commerce/types"
)

// Starter credit constants.
const (
	StarterCreditCents = 500        // $5.00 USD
	StarterCreditDays  = 30         // expires in 30 days
	StarterCreditTag   = "starter-credit"
)

type depositRequest struct {
	User      string `json:"user"`
	Currency  string `json:"currency"`
	Amount    int64  `json:"amount"`    // cents
	Notes     string `json:"notes"`
	Tags      string `json:"tags"`
	ExpiresIn int    `json:"expiresIn"` // days until expiry (0 = no expiry)
}

// Deposit creates a deposit (credit) transaction for an IAM user.
//
//	POST /api/v1/billing/deposit
//
// Used by internal services to add funds to a user's account (payment
// processor settlement, manual credit, promotional grants, etc.).
func Deposit(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	var req depositRequest
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

	cur := currency.Type(strings.ToLower(req.Currency))
	if cur == "" {
		cur = "usd"
	}

	notes := req.Notes
	if notes == "" {
		notes = fmt.Sprintf("Deposit: %d cents %s", req.Amount, cur)
	}

	trans := transaction.New(db)
	trans.Type = transaction.Deposit
	trans.DestinationId = req.User
	trans.DestinationKind = "iam-user"
	trans.Currency = cur
	trans.Amount = currency.Cents(req.Amount)
	trans.Notes = notes
	trans.Tags = req.Tags

	if req.ExpiresIn > 0 {
		trans.ExpiresAt = time.Now().AddDate(0, 0, req.ExpiresIn)
	}

	if !org.Live {
		trans.Test = true
	}

	if err := trans.Create(); err != nil {
		log.Error("Failed to create deposit transaction: %v", err, c)
		http.Fail(c, 500, "failed to create deposit", err)
		return
	}

	resp := gin.H{
		"transactionId": trans.Id(),
		"user":          req.User,
		"amount":        req.Amount,
		"currency":      cur,
		"type":          "deposit",
		"tags":          req.Tags,
	}
	if !trans.ExpiresAt.IsZero() {
		resp["expiresAt"] = trans.ExpiresAt
	}

	c.JSON(201, resp)
}

// GrantStarterCredit creates a $5 USD starter credit for a new user.
// The credit expires after 30 days if unused. Tagged "starter-credit"
// so it can be identified in transaction history.
//
//	POST /api/v1/billing/credit
func GrantStarterCredit(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	var req struct {
		User string `json:"user"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		http.Fail(c, 400, "invalid request body", err)
		return
	}

	if req.User == "" {
		http.Fail(c, 400, "user is required", nil)
		return
	}

	trans := transaction.New(db)
	trans.Type = transaction.Deposit
	trans.DestinationId = req.User
	trans.DestinationKind = "iam-user"
	trans.Currency = "usd"
	trans.Amount = currency.Cents(StarterCreditCents)
	trans.Notes = "Welcome credit: $5.00 USD (expires in 30 days)"
	trans.Tags = StarterCreditTag
	trans.ExpiresAt = time.Now().AddDate(0, 0, StarterCreditDays)
	trans.Metadata = Map{
		"creditType": "starter",
		"expiryDays": StarterCreditDays,
	}

	if !org.Live {
		trans.Test = true
	}

	if err := trans.Create(); err != nil {
		log.Error("Failed to grant starter credit: %v", err, c)
		http.Fail(c, 500, "failed to grant starter credit", err)
		return
	}

	c.JSON(201, gin.H{
		"transactionId": trans.Id(),
		"user":          req.User,
		"amount":        StarterCreditCents,
		"currency":      "usd",
		"type":          "deposit",
		"tags":          StarterCreditTag,
		"expiresAt":     trans.ExpiresAt,
	})
}
