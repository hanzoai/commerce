package billing

import (
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/meter"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"
)

type createMeterRequest struct {
	Name            string   `json:"name"`
	EventName       string   `json:"eventName"`
	AggregationType string   `json:"aggregationType"`
	Currency        string   `json:"currency"`
	Dimensions      []string `json:"dimensions"`
}

// CreateMeter creates a new usage meter definition.
//
//	POST /v1/billing/meters
func CreateMeter(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	var req createMeterRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	if req.Name == "" {
		return http.Fail(c, 400, "name is required", nil)
	}

	if req.EventName == "" {
		return http.Fail(c, 400, "eventName is required", nil)
	}

	aggType := meter.AggregationType(strings.ToLower(req.AggregationType))
	if aggType == "" {
		aggType = meter.AggSum
	}

	cur := currency.Type(strings.ToLower(req.Currency))
	if cur == "" {
		cur = "usd"
	}

	m := meter.New(db)
	m.Name = req.Name
	m.EventName = req.EventName
	m.AggregationType = aggType
	m.Currency = cur
	m.Dimensions = req.Dimensions

	if err := m.Create(); err != nil {
		log.Error("Failed to create meter: %v", err, c)
		return http.Fail(c, 500, "failed to create meter", err)
	}

	return c.JSON(201, map[string]any{
		"id":              m.Id(),
		"name":            m.Name,
		"eventName":       m.EventName,
		"aggregationType": m.AggregationType,
		"currency":        m.Currency,
		"dimensions":      m.Dimensions,
		"createdAt":       m.CreatedAt,
	})
}

// ListMeters returns all meters for the organization.
//
//	GET /v1/billing/meters
func ListMeters(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	rootKey := db.NewKey("synckey", "", 1, nil)

	meters := make([]*meter.Meter, 0)
	q := meter.Query(db).Ancestor(rootKey)

	if _, err := q.GetAll(&meters); err != nil {
		log.Error("Failed to list meters: %v", err, c)
		return http.Fail(c, 500, "failed to list meters", err)
	}

	items := make([]map[string]any, 0, len(meters))
	for _, m := range meters {
		items = append(items, map[string]any{
			"id":              m.Id(),
			"name":            m.Name,
			"eventName":       m.EventName,
			"aggregationType": m.AggregationType,
			"currency":        m.Currency,
			"dimensions":      m.Dimensions,
			"createdAt":       m.CreatedAt,
		})
	}

	return c.JSON(200, map[string]any{
		"meters": items,
		"count":  len(items),
	})
}

// GetMeter returns a single meter by ID.
//
//	GET /v1/billing/meters/:id
func GetMeter(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	id := c.Param("id")
	if id == "" {
		return http.Fail(c, 400, "meter id is required", nil)
	}

	m := meter.New(db)
	if err := m.GetById(id); err != nil {
		return http.Fail(c, 404, "meter not found", err)
	}

	return c.JSON(200, map[string]any{
		"id":              m.Id(),
		"name":            m.Name,
		"eventName":       m.EventName,
		"aggregationType": m.AggregationType,
		"currency":        m.Currency,
		"dimensions":      m.Dimensions,
		"createdAt":       m.CreatedAt,
	})
}
