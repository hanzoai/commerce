package billing

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/billing/credit"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"

	. "github.com/hanzoai/commerce/types"
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
//	POST /v1/billing/deposit
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

	req.User = strings.ToLower(strings.TrimSpace(req.User))
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

// GrantStarterCredit creates a $100 USD starter credit for a new org.
// The credit expires after 365 days if unused. Tagged "starter-credit"
// so it can be identified in transaction history.
//
// No payment method is required — the starter credit is the on-signup grant
// that lets a new org evaluate the platform before adding a card. A verified
// payment method is required only to top up BEYOND the starter credit.
// Idempotent: deduped by the starter-credit tag.
//
//	POST /v1/billing/credit
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

	req.User = strings.ToLower(strings.TrimSpace(req.User))
	if req.User == "" {
		http.Fail(c, 400, "user is required", nil)
		return
	}

	rootKey := db.NewKey("synckey", "", 1, nil)

	// No payment-method gate: the starter credit is grantable WITHOUT a card.
	// A verified payment method is required only for top-up beyond the starter
	// credit (see TopupWithToken/Topup). The tag dedupe below still prevents
	// double-dipping.

	// Check if starter credit was already granted (prevent double-dipping).
	existingTrans := make([]*transaction.Transaction, 0)
	tq := transaction.Query(db).Ancestor(rootKey).
		Filter("DestinationId=", req.User).
		Filter("Tags=", credit.StarterCreditTag)
	if _, err := tq.Limit(1).GetAll(&existingTrans); err == nil && len(existingTrans) > 0 {
		http.Fail(c, 409, "starter credit already granted", nil)
		return
	}

	trans := transaction.New(db)
	trans.Type = transaction.Deposit
	trans.DestinationId = req.User
	trans.DestinationKind = "iam-user"
	trans.Currency = "usd"
	trans.Amount = currency.Cents(credit.StarterCreditCents)
	trans.Notes = "Welcome credit: $100.00 USD (expires in 365 days)"
	trans.Tags = credit.StarterCreditTag
	trans.ExpiresAt = time.Now().AddDate(0, 0, credit.StarterCreditDays)
	trans.Metadata = Map{
		"creditType": "starter",
		"expiryDays": credit.StarterCreditDays,
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
		"amount":        credit.StarterCreditCents,
		"currency":      "usd",
		"type":          "deposit",
		"tags":          credit.StarterCreditTag,
		"expiresAt":     trans.ExpiresAt,
	})
}
