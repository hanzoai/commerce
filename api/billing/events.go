package billing

import (

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/util/json/http"
)

// ListBillingEvents lists billing events, optionally filtered by type or objectId.
//
//	GET /v1/billing/events?type=...&objectId=...
func ListBillingEvents(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	rootKey := db.NewKey("synckey", "", 1, nil)
	events := make([]*billingevent.BillingEvent, 0)
	q := billingevent.Query(db).Ancestor(rootKey)

	if eventType := c.Query("type"); eventType != "" {
		q = q.Filter("Type=", eventType)
	}
	if objectId := c.Query("objectId"); objectId != "" {
		q = q.Filter("ObjectId=", objectId)
	}

	iter := q.Order("-Created").Limit(100).Run()
	for {
		evt := billingevent.New(db)
		if _, err := iter.Next(evt); err != nil {
			break
		}
		events = append(events, evt)
	}

	results := make([]map[string]interface{}, len(events))
	for i, evt := range events {
		results[i] = billingEventResponse(evt)
	}
	return c.JSON(200, results)
}

// GetBillingEvent retrieves a single billing event.
//
//	GET /v1/billing/events/:id
func GetBillingEvent(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	evt := billingevent.New(db)
	if err := evt.GetById(c.Param("id")); err != nil {
		return http.Fail(c, 404, "billing event not found", err)
	}

	return c.JSON(200, billingEventResponse(evt))
}

type createWebhookEndpointRequest struct {
	Url         string   `json:"url"`
	Events      []string `json:"events,omitempty"`
	Description string   `json:"description,omitempty"`
}

func billingEventResponse(evt *billingevent.BillingEvent) map[string]interface{} {
	resp := map[string]interface{}{
		"id":         evt.Id(),
		"type":       evt.Type,
		"objectType": evt.ObjectType,
		"objectId":   evt.ObjectId,
		"livemode":   evt.Livemode,
		"pending":    evt.Pending,
		"created":    evt.Created,
	}
	if evt.CustomerId != "" {
		resp["customerId"] = evt.CustomerId
	}
	if evt.Data != nil {
		resp["data"] = evt.Data
	}
	if evt.PreviousData != nil {
		resp["previousData"] = evt.PreviousData
	}
	return resp
}
