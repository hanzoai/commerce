package billing

import (
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/json/http"
)

type cycleUserRequest struct {
	UserId string `json:"userId"`
}

type cycleResult struct {
	UserId         string `json:"userId"`
	SubscriptionId string `json:"subscriptionId"`
	InvoiceId      string `json:"invoiceId"`
	Success        bool   `json:"success"`
	Error          string `json:"error,omitempty"`
}

// RunBillingCycle processes all subscriptions whose current period has ended
// for the request's organization. It generates invoices and attempts collection
// for each due subscription.
//
//	POST /v1/billing/cycle/run
func RunBillingCycle(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	results := renewDueSubscriptions(c, db)

	// Top up each active subscriber's included monthly allotment for the
	// current period (idempotent per user+month).
	allotGranted, allotSkipped, _ := grantOrgAllotments(c, db, time.Now(), !org.TestMode())

	return c.JSON(200, map[string]any{
		"processed": len(results),
		"results":   results,
		"allotments": map[string]any{
			"granted": allotGranted,
			"skipped": allotSkipped,
		},
	})
}

// RunBillingCycleUser processes due subscriptions for a single user within the
// request's organization.
//
//	POST /v1/billing/cycle/run-user
func RunBillingCycleUser(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	var req cycleUserRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	if req.UserId == "" {
		return http.Fail(c, 400, "userId is required", nil)
	}

	results := renewDueSubscriptionsForUser(c, db, req.UserId)

	return c.JSON(200, map[string]any{
		"user":      req.UserId,
		"processed": len(results),
		"results":   results,
	})
}

// RunBillingCycleAllOrgs iterates every organization and processes due
// subscriptions across all of them. This is intended for the platform
// scheduler to invoke on a recurring basis.
//
//	POST /v1/billing/cycle/run-all
func RunBillingCycleAllOrgs(c *zip.Ctx) error {
	rootDb := datastore.New(c.Context())

	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(rootDb).GetAll(&orgs); err != nil {
		log.Error("Failed to list organizations for billing cycle: %v", err, c)
		return http.Fail(c, 500, "failed to list organizations", err)
	}

	type orgResult struct {
		OrgId     string        `json:"orgId"`
		OrgName   string        `json:"orgName"`
		Processed int           `json:"processed"`
		Results   []cycleResult `json:"results"`
	}

	allResults := make([]orgResult, 0, len(orgs))
	totalProcessed := 0

	now := time.Now()
	for _, org := range orgs {
		db := datastore.New(org.Namespaced(c.Context()))
		results := renewDueSubscriptions(c, db)
		totalProcessed += len(results)

		// Grant included monthly allotment for active subscribers (idempotent).
		grantOrgAllotments(c, db, now, !org.TestMode())

		if len(results) > 0 {
			allResults = append(allResults, orgResult{
				OrgId:     org.Id(),
				OrgName:   org.Name,
				Processed: len(results),
				Results:   results,
			})
		}
	}

	return c.JSON(200, map[string]any{
		"orgs":           len(allResults),
		"totalProcessed": totalProcessed,
		"results":        allResults,
	})
}

// renewDueSubscriptions finds all active or past-due subscriptions whose
// current period has ended and renews each one. Returns a result per
// subscription processed.
func renewDueSubscriptions(c *zip.Ctx, db *datastore.Datastore) []cycleResult {
	now := time.Now()
	rootKey := db.NewKey("synckey", "", 1, nil)

	subs := make([]*subscription.Subscription, 0)
	q := subscription.Query(db).Ancestor(rootKey)

	if _, err := q.GetAll(&subs); err != nil {
		log.Error("Failed to query subscriptions for billing cycle: %v", err, c)
		return nil
	}

	results := make([]cycleResult, 0)
	for _, sub := range subs {
		if !isDueForRenewal(sub, now) {
			continue
		}
		results = append(results, renewOne(c, db, sub))
	}

	return results
}

// renewDueSubscriptionsForUser is the same as renewDueSubscriptions but
// scoped to a single user.
func renewDueSubscriptionsForUser(c *zip.Ctx, db *datastore.Datastore, userId string) []cycleResult {
	now := time.Now()
	rootKey := db.NewKey("synckey", "", 1, nil)

	subs := make([]*subscription.Subscription, 0)
	q := subscription.Query(db).Ancestor(rootKey).Filter("UserId=", userId)

	if _, err := q.GetAll(&subs); err != nil {
		log.Error("Failed to query subscriptions for user %s: %v", userId, err, c)
		return nil
	}

	results := make([]cycleResult, 0)
	for _, sub := range subs {
		if !isDueForRenewal(sub, now) {
			continue
		}
		results = append(results, renewOne(c, db, sub))
	}

	return results
}

// isDueForRenewal returns true when a subscription's current billing period
// has elapsed and the subscription is eligible for renewal.
func isDueForRenewal(sub *subscription.Subscription, now time.Time) bool {
	switch sub.Status {
	case subscription.Active, subscription.PastDue:
		return !sub.PeriodEnd.IsZero() && now.After(sub.PeriodEnd)
	default:
		return false
	}
}

// renewOne generates an invoice and attempts collection for a single
// subscription, then persists the updated subscription state.
func renewOne(c *zip.Ctx, db *datastore.Datastore, sub *subscription.Subscription) cycleResult {
	inv, result, err := engine.RenewSubscription(c.Context(), db, sub, BurnCredits)
	if err != nil {
		log.Error("Billing cycle: failed to renew subscription %s: %v", sub.Id(), err, c)
		return cycleResult{
			UserId:         sub.UserId,
			SubscriptionId: sub.Id(),
			Success:        false,
			Error:          err.Error(),
		}
	}

	if err := sub.Update(); err != nil {
		log.Error("Billing cycle: failed to update subscription %s after renewal: %v", sub.Id(), err, c)
		return cycleResult{
			UserId:         sub.UserId,
			SubscriptionId: sub.Id(),
			InvoiceId:      inv.Id(),
			Success:        false,
			Error:          err.Error(),
		}
	}

	return cycleResult{
		UserId:         sub.UserId,
		SubscriptionId: sub.Id(),
		InvoiceId:      inv.Id(),
		Success:        result.Success,
	}
}
