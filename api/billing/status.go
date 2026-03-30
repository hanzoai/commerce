package billing

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"
)

// GetBillingStatus returns a unified billing status for a user.
// Used by the bot gateway billing-gate to decide whether to allow LLM requests.
//
//	GET /api/v1/billing/status?user=<userId>
//
// Response:
//
//	{
//	  "user": "alice",
//	  "hasPaymentMethod": true,
//	  "creditBalance": 500,   // cents
//	  "tier": "developer"
//	}
func GetBillingStatus(c *gin.Context) {
	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c)
	db := datastore.New(ctx)

	user := strings.TrimSpace(c.Query("user"))
	if user == "" {
		// Fall back to the authenticated IAM user from context
		if email := strings.TrimSpace(c.GetString("iam_email")); email != "" {
			user = email
		} else if sub := strings.TrimSpace(c.GetString("iam_user_id")); sub != "" {
			user = sub
		}
	}
	if user == "" {
		http.Fail(c, 400, "user query parameter is required", nil)
		return
	}

	// 1. Check whether the user has at least one active payment method.
	rootKey := db.NewKey("synckey", "", 1, nil)
	iter := paymentmethod.Query(db).Ancestor(rootKey).
		Filter("CustomerId=", user).
		Limit(1).
		Run()

	hasPaymentMethod := false
	probe := paymentmethod.New(db)
	if _, err := iter.Next(probe); err == nil {
		hasPaymentMethod = true
	}

	// 2. Get available credit balance (USD).
	var creditBalance currency.Cents
	datas, err := util.GetTransactionsByCurrency(ctx, user, "iam-user", currency.USD, !org.Live)
	if err == nil {
		if data, ok := datas.Data[currency.USD]; ok {
			avail := data.Balance - data.Holds
			if avail > 0 {
				creditBalance = avail
			}
		}
	}

	c.JSON(200, gin.H{
		"user":             user,
		"hasPaymentMethod": hasPaymentMethod,
		"creditBalance":    creditBalance,
	})
}
