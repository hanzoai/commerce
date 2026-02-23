package billing

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/json/http"
)

type createSubscriptionRequest struct {
	UserId               string                 `json:"userId"`
	PlanId               string                 `json:"planId"`
	DefaultPaymentMethod string                 `json:"defaultPaymentMethod"`
	Metadata             map[string]interface{} `json:"metadata"`
}

type updateSubscriptionRequest struct {
	PlanId   string `json:"planId"`
	Prorate  bool   `json:"prorate"`
	Quantity int    `json:"quantity"`
}

type cancelSubscriptionRequest struct {
	AtPeriodEnd bool `json:"atPeriodEnd"`
}

// CreateBillingSubscription creates a new subscription and starts the billing lifecycle.
//
//	POST /api/v1/billing/subscriptions
func CreateBillingSubscription(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	var req createSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		http.Fail(c, 400, "invalid request body", err)
		return
	}

	if req.UserId == "" {
		http.Fail(c, 400, "userId is required", nil)
		return
	}

	if req.PlanId == "" {
		http.Fail(c, 400, "planId is required", nil)
		return
	}

	// Fetch plan
	p := plan.New(db)
	if err := p.GetById(req.PlanId); err != nil {
		http.Fail(c, 404, "plan not found", err)
		return
	}

	// Create subscription
	sub := subscription.New(db)
	sub.UserId = req.UserId
	sub.DefaultPaymentMethod = req.DefaultPaymentMethod
	sub.ProviderType = "internal"
	sub.Quantity = 1

	if req.Metadata != nil {
		sub.Metadata = req.Metadata
	}

	// Initialize subscription lifecycle
	engine.StartSubscription(sub, p)

	if err := sub.Create(); err != nil {
		log.Error("Failed to create subscription: %v", err, c)
		http.Fail(c, 500, "failed to create subscription", err)
		return
	}

	c.JSON(201, subscriptionResponse(sub))
}

// ListBillingSubscriptions lists subscriptions for a user.
//
//	GET /api/v1/billing/subscriptions?userId=...
func ListBillingSubscriptions(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	rootKey := db.NewKey("synckey", "", 1, nil)
	subs := make([]*subscription.Subscription, 0)
	q := subscription.Query(db).Ancestor(rootKey)

	userId := strings.TrimSpace(c.Query("userId"))
	if userId != "" {
		q = q.Filter("UserId=", userId)
	}

	status := strings.TrimSpace(c.Query("status"))
	if status != "" {
		q = q.Filter("Status=", status)
	}

	if _, err := q.GetAll(&subs); err != nil {
		log.Error("Failed to list subscriptions: %v", err, c)
		http.Fail(c, 500, "failed to list subscriptions", err)
		return
	}

	items := make([]gin.H, 0, len(subs))
	for _, s := range subs {
		items = append(items, subscriptionResponse(s))
	}

	c.JSON(200, gin.H{
		"subscriptions": items,
		"count":         len(items),
	})
}

// GetBillingSubscription returns a single subscription.
//
//	GET /api/v1/billing/subscriptions/:id
func GetBillingSubscription(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	id := c.Param("id")
	sub := subscription.New(db)
	if err := sub.GetById(id); err != nil {
		http.Fail(c, 404, "subscription not found", err)
		return
	}

	c.JSON(200, subscriptionResponse(sub))
}

// UpdateBillingSubscription updates a subscription (plan change, quantity).
//
//	PATCH /api/v1/billing/subscriptions/:id
func UpdateBillingSubscription(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	id := c.Param("id")
	sub := subscription.New(db)
	if err := sub.GetById(id); err != nil {
		http.Fail(c, 404, "subscription not found", err)
		return
	}

	var req updateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		http.Fail(c, 400, "invalid request body", err)
		return
	}

	if req.PlanId != "" && req.PlanId != sub.PlanId {
		// Fetch new plan
		newPlan := plan.New(db)
		if err := newPlan.GetById(req.PlanId); err != nil {
			http.Fail(c, 404, "new plan not found", err)
			return
		}

		_, err := engine.ChangePlan(sub, newPlan, req.Prorate)
		if err != nil {
			http.Fail(c, 400, err.Error(), nil)
			return
		}
	}

	if req.Quantity > 0 {
		sub.Quantity = req.Quantity
	}

	if err := sub.Update(); err != nil {
		log.Error("Failed to update subscription: %v", err, c)
		http.Fail(c, 500, "failed to update subscription", err)
		return
	}

	c.JSON(200, subscriptionResponse(sub))
}

// CancelBillingSubscription cancels a subscription.
//
//	POST /api/v1/billing/subscriptions/:id/cancel
func CancelBillingSubscription(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	id := c.Param("id")
	sub := subscription.New(db)
	if err := sub.GetById(id); err != nil {
		http.Fail(c, 404, "subscription not found", err)
		return
	}

	var req cancelSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Default to cancel at period end
		req.AtPeriodEnd = true
	}

	if err := engine.CancelSubscription(sub, req.AtPeriodEnd); err != nil {
		http.Fail(c, 400, err.Error(), nil)
		return
	}

	if err := sub.Update(); err != nil {
		log.Error("Failed to cancel subscription: %v", err, c)
		http.Fail(c, 500, "failed to cancel subscription", err)
		return
	}

	c.JSON(200, subscriptionResponse(sub))
}

// ReactivateBillingSubscription reactivates a canceled subscription.
//
//	POST /api/v1/billing/subscriptions/:id/reactivate
func ReactivateBillingSubscription(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	id := c.Param("id")
	sub := subscription.New(db)
	if err := sub.GetById(id); err != nil {
		http.Fail(c, 404, "subscription not found", err)
		return
	}

	if err := engine.ReactivateSubscription(sub); err != nil {
		http.Fail(c, 400, err.Error(), nil)
		return
	}

	if err := sub.Update(); err != nil {
		log.Error("Failed to reactivate subscription: %v", err, c)
		http.Fail(c, 500, "failed to reactivate subscription", err)
		return
	}

	c.JSON(200, subscriptionResponse(sub))
}

// RenewBillingSubscription manually triggers a billing cycle renewal.
// Normally this would be automated by Temporal, but this endpoint allows
// manual triggering for testing and for deployments without Temporal.
//
//	POST /api/v1/billing/subscriptions/:id/renew
func RenewBillingSubscription(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	id := c.Param("id")
	sub := subscription.New(db)
	if err := sub.GetById(id); err != nil {
		http.Fail(c, 404, "subscription not found", err)
		return
	}

	inv, result, err := engine.RenewSubscription(c.Request.Context(), db, sub, BurnCredits)
	if err != nil {
		log.Error("Failed to renew subscription: %v", err, c)
		http.Fail(c, 500, "failed to renew subscription", err)
		return
	}

	if err := sub.Update(); err != nil {
		log.Error("Failed to update subscription after renewal: %v", err, c)
		http.Fail(c, 500, "failed to update subscription", err)
		return
	}

	c.JSON(200, gin.H{
		"subscription": subscriptionResponse(sub),
		"invoice":      invoiceResponse(inv),
		"collection":   result,
	})
}

func subscriptionResponse(sub *subscription.Subscription) gin.H {
	resp := gin.H{
		"id":                   sub.Id(),
		"userId":               sub.UserId,
		"planId":               sub.PlanId,
		"status":               sub.Status,
		"quantity":             sub.Quantity,
		"currentPeriodStart":   sub.PeriodStart,
		"currentPeriodEnd":     sub.PeriodEnd,
		"cancelAtPeriodEnd":    sub.EndCancel,
		"providerType":         sub.ProviderType,
		"defaultPaymentMethod": sub.DefaultPaymentMethod,
		"plan": gin.H{
			"id":       sub.Plan.Id(),
			"name":     sub.Plan.Name,
			"price":    sub.Plan.Price,
			"currency": sub.Plan.Currency,
			"interval": sub.Plan.Interval,
		},
		"createdAt": sub.CreatedAt,
		"updatedAt": sub.UpdatedAt,
	}

	if !sub.TrialStart.IsZero() {
		resp["trialStart"] = sub.TrialStart
		resp["trialEnd"] = sub.TrialEnd
	}
	if !sub.CanceledAt.IsZero() {
		resp["canceledAt"] = sub.CanceledAt
	}
	if !sub.Ended.IsZero() {
		resp["endedAt"] = sub.Ended
	}

	return resp
}
