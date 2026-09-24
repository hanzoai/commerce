package billing

import (
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/subscriptionschedule"
	"github.com/hanzoai/commerce/util/json/http"
)

type createSchedulePhaseItem struct {
	PriceId  string `json:"priceId"`
	Quantity int64  `json:"quantity"`
}

type createSchedulePhase struct {
	PlanId            string                    `json:"planId"`
	Items             []createSchedulePhaseItem `json:"items,omitempty"`
	StartDate         time.Time                 `json:"startDate"`
	EndDate           time.Time                 `json:"endDate"`
	TrialEnd          time.Time                 `json:"trialEnd,omitempty"`
	ProrationBehavior string                    `json:"prorationBehavior,omitempty"`
}

type createSubscriptionScheduleRequest struct {
	CustomerId  string                 `json:"customerId"`
	StartDate   time.Time              `json:"startDate"`
	EndBehavior string                 `json:"endBehavior,omitempty"`
	Phases      []createSchedulePhase  `json:"phases,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type updateSubscriptionScheduleRequest struct {
	EndBehavior string                `json:"endBehavior,omitempty"`
	Phases      []createSchedulePhase `json:"phases,omitempty"`
}

// CreateSubscriptionSchedule creates a new subscription schedule.
//
//	POST /v1/billing/subscription-schedules
func CreateSubscriptionSchedule(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	var req createSubscriptionScheduleRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	if req.CustomerId == "" {
		return http.Fail(c, 400, "customerId is required", nil)
	}

	s := subscriptionschedule.New(db)
	s.CustomerId = req.CustomerId
	s.StartDate = req.StartDate
	if req.EndBehavior != "" {
		s.EndBehavior = req.EndBehavior
	}
	if req.Metadata != nil {
		s.Metadata = req.Metadata
	}

	if len(req.Phases) > 0 {
		phases := make([]subscriptionschedule.Phase, len(req.Phases))
		for i, p := range req.Phases {
			items := make([]subscriptionschedule.PhaseItem, len(p.Items))
			for j, item := range p.Items {
				items[j] = subscriptionschedule.PhaseItem{
					PriceId:  item.PriceId,
					Quantity: item.Quantity,
				}
			}
			phases[i] = subscriptionschedule.Phase{
				PlanId:            p.PlanId,
				Items:             items,
				StartDate:         p.StartDate,
				EndDate:           p.EndDate,
				TrialEnd:          p.TrialEnd,
				ProrationBehavior: p.ProrationBehavior,
			}
		}
		s.Phases = phases
	}

	if err := s.Create(); err != nil {
		log.Error("Failed to create subscription schedule: %v", err, c)
		return http.Fail(c, 500, "failed to create subscription schedule", err)
	}

	return c.JSON(201, scheduleResponse(s))
}

// GetSubscriptionSchedule retrieves a subscription schedule by ID.
//
//	GET /v1/billing/subscription-schedules/:id
func GetSubscriptionSchedule(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	s := subscriptionschedule.New(db)
	if err := s.GetById(c.Param("id")); err != nil {
		return http.Fail(c, 404, "subscription schedule not found", err)
	}

	return c.JSON(200, scheduleResponse(s))
}

// ListSubscriptionSchedules lists subscription schedules.
//
//	GET /v1/billing/subscription-schedules?customerId=...&status=...
func ListSubscriptionSchedules(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	rootKey := db.NewKey("synckey", "", 1, nil)
	schedules := make([]*subscriptionschedule.SubscriptionSchedule, 0)
	q := subscriptionschedule.Query(db).Ancestor(rootKey)

	if custId := c.Query("customerId"); custId != "" {
		q = q.Filter("CustomerId=", custId)
	}
	if status := c.Query("status"); status != "" {
		q = q.Filter("Status=", status)
	}

	iter := q.Order("-Created").Run()
	for {
		s := subscriptionschedule.New(db)
		if _, err := iter.Next(s); err != nil {
			break
		}
		schedules = append(schedules, s)
	}

	results := make([]map[string]interface{}, len(schedules))
	for i, s := range schedules {
		results[i] = scheduleResponse(s)
	}
	return c.JSON(200, results)
}

// UpdateSubscriptionSchedule updates phases or end behavior.
//
//	PATCH /v1/billing/subscription-schedules/:id
func UpdateSubscriptionSchedule(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	s := subscriptionschedule.New(db)
	if err := s.GetById(c.Param("id")); err != nil {
		return http.Fail(c, 404, "subscription schedule not found", err)
	}

	var req updateSubscriptionScheduleRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	if req.EndBehavior != "" {
		s.EndBehavior = req.EndBehavior
	}

	if len(req.Phases) > 0 {
		phases := make([]subscriptionschedule.Phase, len(req.Phases))
		for i, p := range req.Phases {
			items := make([]subscriptionschedule.PhaseItem, len(p.Items))
			for j, item := range p.Items {
				items[j] = subscriptionschedule.PhaseItem{
					PriceId:  item.PriceId,
					Quantity: item.Quantity,
				}
			}
			phases[i] = subscriptionschedule.Phase{
				PlanId:            p.PlanId,
				Items:             items,
				StartDate:         p.StartDate,
				EndDate:           p.EndDate,
				TrialEnd:          p.TrialEnd,
				ProrationBehavior: p.ProrationBehavior,
			}
		}
		s.Phases = phases
	}

	if err := s.Update(); err != nil {
		log.Error("Failed to update subscription schedule: %v", err, c)
		return http.Fail(c, 500, "failed to update subscription schedule", err)
	}

	return c.JSON(200, scheduleResponse(s))
}

// CancelSubscriptionSchedule cancels a subscription schedule.
//
//	POST /v1/billing/subscription-schedules/:id/cancel
func CancelSubscriptionSchedule(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	s := subscriptionschedule.New(db)
	if err := s.GetById(c.Param("id")); err != nil {
		return http.Fail(c, 404, "subscription schedule not found", err)
	}

	if err := s.Cancel(); err != nil {
		return http.Fail(c, 400, err.Error(), err)
	}

	if err := s.Update(); err != nil {
		log.Error("Failed to cancel subscription schedule: %v", err, c)
		return http.Fail(c, 500, "failed to cancel subscription schedule", err)
	}

	return c.JSON(200, scheduleResponse(s))
}

// ReleaseSubscriptionSchedule releases a subscription schedule.
//
//	POST /v1/billing/subscription-schedules/:id/release
func ReleaseSubscriptionSchedule(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	s := subscriptionschedule.New(db)
	if err := s.GetById(c.Param("id")); err != nil {
		return http.Fail(c, 404, "subscription schedule not found", err)
	}

	if err := s.Release(); err != nil {
		return http.Fail(c, 400, err.Error(), err)
	}

	if err := s.Update(); err != nil {
		log.Error("Failed to release subscription schedule: %v", err, c)
		return http.Fail(c, 500, "failed to release subscription schedule", err)
	}

	return c.JSON(200, scheduleResponse(s))
}

func scheduleResponse(s *subscriptionschedule.SubscriptionSchedule) map[string]interface{} {
	resp := map[string]interface{}{
		"id":          s.Id(),
		"customerId":  s.CustomerId,
		"status":      s.Status,
		"startDate":   s.StartDate,
		"endBehavior": s.EndBehavior,
		"created":     s.Created,
	}
	if s.SubscriptionId != "" {
		resp["subscriptionId"] = s.SubscriptionId
	}
	if len(s.Phases) > 0 {
		resp["phases"] = s.Phases
	}
	if s.Metadata != nil {
		resp["metadata"] = s.Metadata
	}
	return resp
}
