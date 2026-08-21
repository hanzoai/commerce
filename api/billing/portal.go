package billing

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/thirdparty/kms"
	"github.com/hanzoai/commerce/util/json/http"
)

// PortalOverview returns a billing summary for the authenticated customer.
//
//	GET /v1/billing/portal/overview?customerId=...
func PortalOverview(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	customerId := c.Query("customerId")
	if customerId == "" {
		return http.Fail(c, 400, "customerId is required", nil)
	}

	// Get balance
	cb, _ := engine.GetOrCreateCustomerBalance(db, customerId, "usd")
	balance := int64(0)
	if cb != nil {
		balance = cb.Balance
	}

	// Count active subscriptions
	rootKey := db.NewKey("synckey", "", 1, nil)
	subs := make([]*subscription.Subscription, 0)
	sq := subscription.Query(db).Ancestor(rootKey).
		Filter("UserId=", customerId).
		Filter("Status=", string(subscription.Active))
	_, _ = sq.GetAll(&subs)

	// Count payment methods
	pms := make([]*paymentmethod.PaymentMethod, 0)
	pmq := paymentmethod.Query(db).Ancestor(rootKey).
		Filter("CustomerId=", customerId)
	_, _ = pmq.GetAll(&pms)

	return c.JSON(200, map[string]any{
		"customerId":          customerId,
		"balance":             balance,
		"currency":            "usd",
		"activeSubscriptions": len(subs),
		"paymentMethods":      len(pms),
	})
}

// PortalInvoices returns the customer's invoice list.
//
//	GET /v1/billing/portal/invoices?customerId=...
func PortalInvoices(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	customerId := c.Query("customerId")
	if customerId == "" {
		return http.Fail(c, 400, "customerId is required", nil)
	}

	rootKey := db.NewKey("synckey", "", 1, nil)
	invoices := make([]*billinginvoice.BillingInvoice, 0)
	q := billinginvoice.Query(db).Ancestor(rootKey).
		Filter("UserId=", customerId).
		Order("-Created")
	_, _ = q.GetAll(&invoices)

	results := make([]map[string]any, len(invoices))
	for i, inv := range invoices {
		results[i] = map[string]any{
			"id":        inv.Id(),
			"number":    inv.NumberStr,
			"status":    inv.Status,
			"amountDue": inv.AmountDue,
			"currency":  inv.Currency,
			"created":   inv.Created,
		}
	}
	return c.JSON(200, results)
}

// PortalSubscriptions returns the customer's subscriptions.
//
//	GET /v1/billing/portal/subscriptions?customerId=...
func PortalSubscriptions(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	customerId := c.Query("customerId")
	if customerId == "" {
		return http.Fail(c, 400, "customerId is required", nil)
	}

	rootKey := db.NewKey("synckey", "", 1, nil)
	subs := make([]*subscription.Subscription, 0)
	q := subscription.Query(db).Ancestor(rootKey).
		Filter("UserId=", customerId).
		Order("-Created")
	_, _ = q.GetAll(&subs)

	results := make([]map[string]any, len(subs))
	for i, s := range subs {
		results[i] = map[string]any{
			"id":          s.Id(),
			"planId":      s.PlanId,
			"status":      s.Status,
			"periodStart": s.PeriodStart,
			"periodEnd":   s.PeriodEnd,
			"created":     s.Created,
		}
	}
	return c.JSON(200, results)
}

// PortalPaymentMethods returns the customer's payment methods.
//
// The customer named here is a value the CALLER supplies, so the org namespace is
// what keeps one host's customerId from selecting another org's cards; asking for
// a foreign subject selects nothing inside your own namespace. It answers with
// the same ListMethods every other saved-card surface reads, because a portal
// that derived its own list would be a second answer to one question.
//
//	GET /v1/billing/portal/methods?customerId=...
func PortalPaymentMethods(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)

	customerId := c.Query("customerId")
	if customerId == "" {
		return http.Fail(c, 400, "customerId is required", nil)
	}

	creds, _ := c.Locals("kms").(*kms.CachedClient)
	methods, err := ListMethods(c.Context(), org, customerId, "", creds)
	if err != nil {
		// A store that cannot be read has always answered an honest empty list
		// here rather than a 500 — the portal renders no cards, not an error page.
		log.Error("Failed to list portal payment methods: %v", err, c)
	}
	return c.JSON(200, methods)
}
