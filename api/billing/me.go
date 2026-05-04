package billing

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/billing/credit"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/pkg/auth"
	"github.com/hanzoai/commerce/util/json/http"
)

// userKey returns the canonical "<org>/<user>" key used as the
// destination identifier for IAM-bound transactions, derived from the
// gateway-injected trust headers. Returns "" when identity is missing —
// callers should 401.
func userKey(c *gin.Context) string {
	ctx := c.Request.Context()
	org := strings.ToLower(strings.TrimSpace(auth.OrgID(ctx)))
	uid := strings.ToLower(strings.TrimSpace(auth.UserID(ctx)))
	if org == "" || uid == "" {
		return ""
	}
	return org + "/" + uid
}

// GetMyBalance returns the calling user's balance for a given currency.
// Identity comes from the gateway-injected X-Org-Id / X-User-Id headers;
// no admin token required.
//
//	GET /api/v1/billing/me/balance?currency=usd
func GetMyBalance(c *gin.Context) {
	user := userKey(c)
	if user == "" {
		http.Fail(c, 401, "missing identity headers", nil)
		return
	}

	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c)

	cur := currency.Type(strings.ToLower(c.DefaultQuery("currency", "usd")))

	datas, err := util.GetTransactionsByCurrency(ctx, user, "iam-user", cur, !org.Live)
	if err != nil {
		http.Fail(c, 500, "failed to query balance", err)
		return
	}

	var balance, holds currency.Cents
	if data, ok := datas.Data[cur]; ok {
		balance = data.Balance
		holds = data.Holds
	}

	available := balance - holds
	if available < 0 {
		available = 0
	}

	c.JSON(200, gin.H{
		"user":      user,
		"currency":  cur,
		"balance":   int64(balance),
		"holds":     int64(holds),
		"available": int64(available),
	})
}

// PostMyWelcome grants the welcome credit (idempotent, tag-deduped) to
// the calling user. Unlike POST /billing/credit which requires an
// admin token AND a payment method on file, this endpoint is callable
// with just an IAM bearer token — designed to be invoked by the
// playground SPA on first successful login.
//
// Idempotent: if the credit was already granted (or zapped), returns
// 200 with `granted: false` instead of failing.
//
//	POST /api/v1/billing/me/welcome
func PostMyWelcome(c *gin.Context) {
	user := userKey(c)
	if user == "" {
		http.Fail(c, 401, "missing identity headers", nil)
		return
	}

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	rootKey := db.NewKey("synckey", "", 1, nil)

	// Already granted? — return 200 with granted=false.
	existing := make([]*transaction.Transaction, 0)
	q := transaction.Query(db).Ancestor(rootKey).
		Filter("DestinationId=", user).
		Filter("Tags=", credit.StarterCreditTag)
	if _, err := q.Limit(1).GetAll(&existing); err == nil && len(existing) > 0 {
		c.JSON(200, gin.H{
			"user":    user,
			"granted": false,
			"reason":  "already_granted",
		})
		return
	}

	trans := transaction.New(db)
	trans.Type = transaction.Deposit
	trans.DestinationId = user
	trans.DestinationKind = "iam-user"
	trans.Currency = "usd"
	trans.Amount = currency.Cents(credit.StarterCreditCents)
	trans.Notes = "Welcome credit: $5.00 USD (expires in 30 days)"
	trans.Tags = credit.StarterCreditTag
	trans.ExpiresAt = time.Now().AddDate(0, 0, credit.StarterCreditDays)
	trans.Metadata = Map{
		"creditType": "starter",
		"expiryDays": credit.StarterCreditDays,
		"trigger":    "iam_first_login",
	}
	if !org.Live {
		trans.Test = true
	}

	if err := trans.Create(); err != nil {
		log.Error("welcome credit create failed for %s: %v", user, err, c)
		http.Fail(c, 500, "failed to grant welcome credit", err)
		return
	}

	c.JSON(201, gin.H{
		"user":      user,
		"granted":   true,
		"amount":    int64(credit.StarterCreditCents),
		"currency":  "usd",
		"expiresAt": trans.ExpiresAt,
	})
}
