package billing

import (
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/meter"
	"github.com/hanzoai/commerce/util/json/http"

	. "github.com/hanzoai/commerce/types"
)

type meterEventInput struct {
	MeterId     string `json:"meterId"`
	UserId      string `json:"userId"`
	Value       int64  `json:"value"`
	Timestamp   string `json:"timestamp"`
	Idempotency string `json:"idempotency"`
	Dimensions  Map    `json:"dimensions"`
}

type recordEventsRequest struct {
	Events []meterEventInput `json:"events"`
}

// RecordMeterEvents records one or more meter events (batch up to 100).
//
//	POST /v1/billing/meter-events
func RecordMeterEvents(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	var req recordEventsRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	if len(req.Events) == 0 {
		return http.Fail(c, 400, "at least one event is required", nil)
	}

	if len(req.Events) > 100 {
		return http.Fail(c, 400, "maximum 100 events per batch", nil)
	}

	created := make([]map[string]any, 0, len(req.Events))

	for _, input := range req.Events {
		if input.MeterId == "" {
			return http.Fail(c, 400, "meterId is required for each event", nil)
		}
		if input.UserId == "" {
			return http.Fail(c, 400, "userId is required for each event", nil)
		}

		evt := meter.NewEvent(db)
		evt.MeterId = input.MeterId
		evt.UserId = input.UserId
		evt.Value = input.Value
		evt.Idempotency = input.Idempotency
		evt.Dimensions = input.Dimensions

		if input.Timestamp != "" {
			ts, err := time.Parse(time.RFC3339, input.Timestamp)
			if err == nil {
				evt.Timestamp = ts
			}
		}
		if evt.Timestamp.IsZero() {
			evt.Timestamp = time.Now()
		}

		// Dedup by idempotency key if provided
		if input.Idempotency != "" {
			rootKey := db.NewKey("synckey", "", 1, nil)
			existing := make([]*meter.MeterEvent, 0, 1)
			q := meter.QueryEvents(db).Ancestor(rootKey).
				Filter("Idempotency=", input.Idempotency).
				Limit(1)
			if _, err := q.GetAll(&existing); err == nil && len(existing) > 0 {
				// Already recorded — skip silently
				created = append(created, map[string]any{
					"id":     existing[0].Id(),
					"status": "duplicate",
				})
				continue
			}
		}

		if err := evt.Create(); err != nil {
			log.Error("Failed to create meter event: %v", err, c)
			return http.Fail(c, 500, "failed to create meter event", err)
		}

		created = append(created, map[string]any{
			"id":     evt.Id(),
			"status": "created",
		})
	}

	return c.JSON(201, map[string]any{
		"events": created,
		"count":  len(created),
	})
}

// GetMeterEventsSummary returns aggregated usage for a meter+user+period.
//
//	GET /v1/billing/meter-events/summary?meterId=...&userId=...&periodStart=...&periodEnd=...
func GetMeterEventsSummary(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	meterId := strings.TrimSpace(c.Query("meterId"))
	userId := strings.TrimSpace(c.Query("userId"))

	if meterId == "" {
		return http.Fail(c, 400, "meterId query parameter is required", nil)
	}

	// Load the meter to get aggregation type
	m := meter.New(db)
	if err := m.GetById(meterId); err != nil {
		return http.Fail(c, 404, "meter not found", err)
	}

	rootKey := db.NewKey("synckey", "", 1, nil)
	events := make([]*meter.MeterEvent, 0)
	q := meter.QueryEvents(db).Ancestor(rootKey).
		Filter("MeterId=", meterId)

	if userId != "" {
		q = q.Filter("UserId=", userId)
	}

	// Time range filtering
	periodStart := c.Query("periodStart")
	periodEnd := c.Query("periodEnd")
	if periodStart != "" {
		if ts, err := time.Parse(time.RFC3339, periodStart); err == nil {
			q = q.Filter("Timestamp>=", ts)
		} else if ts, err := time.Parse("2006-01-02", periodStart); err == nil {
			q = q.Filter("Timestamp>=", ts)
		}
	}
	if periodEnd != "" {
		if ts, err := time.Parse(time.RFC3339, periodEnd); err == nil {
			q = q.Filter("Timestamp<=", ts)
		} else if ts, err := time.Parse("2006-01-02", periodEnd); err == nil {
			q = q.Filter("Timestamp<=", ts.Add(24*time.Hour))
		}
	}

	if _, err := q.GetAll(&events); err != nil {
		log.Error("Failed to query meter events: %v", err, c)
		return http.Fail(c, 500, "failed to query meter events", err)
	}

	// Aggregate based on meter type
	var aggregatedValue int64
	var eventCount int64

	for _, evt := range events {
		eventCount++
		switch m.AggregationType {
		case meter.AggSum:
			aggregatedValue += evt.Value
		case meter.AggCount:
			aggregatedValue++
		case meter.AggLast:
			aggregatedValue = evt.Value
		default:
			aggregatedValue += evt.Value
		}
	}

	return c.JSON(200, map[string]any{
		"meterId":         meterId,
		"meterName":       m.Name,
		"userId":          userId,
		"aggregationType": m.AggregationType,
		"value":           aggregatedValue,
		"eventCount":      eventCount,
		"periodStart":     periodStart,
		"periodEnd":       periodEnd,
	})
}
