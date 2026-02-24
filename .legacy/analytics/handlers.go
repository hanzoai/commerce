package main

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	// "github.com/hanzoai/commerce/models/aggregate"
	"github.com/hanzoai/commerce/models/analyticsevent"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/types/client"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"

	. "github.com/hanzoai/commerce/models/analyticsidentifier/tasks"
	// . "github.com/hanzoai/commerce/util/aggregate/tasks"
	// . "github.com/hanzoai/commerce/util/analytics/tasks"
)

func create(c *gin.Context) {
	receivedTime := time.Now()

	ctx := middleware.GetContext(c)
	db := datastore.New(ctx)

	id := c.Params.ByName("organizationid")
	org := organization.New(db)
	if err := org.GetById(id); err != nil {
		http.Fail(c, 400, "Failed to get organization", err)
		return
	}

	nsCtx := org.Namespaced(ctx)
	db = datastore.New(nsCtx)

	var events []*analyticsevent.AnalyticsEvent

	if err := json.Decode(c.Request.Body, &events); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	if len(events) > 0 {
		mostRecentTime := events[len(events)-1].Timestamp
		for _, event := range events {
			offset := event.Timestamp.Sub(mostRecentTime)

			event.Init(db)
			event.RequestMetadata = client.New(c)
			event.CalculatedTimestamp = receivedTime.Add(offset)
			if err := event.Put(); err != nil {
				http.Fail(c, 500, "Failed to create event", err)
				return
			}

			// UpsertAggregate(ctx, org.Name, event.Name, "AnalyticsEvent", event.CalculatedTimestamp, aggregate.Hourly, 1, nil)
			// UpdateFunnels(ctx, org.Name, event.Id())
		}

		CohereIds.Call(nsCtx, events[len(events)-1].Id)
	}

	http.Render(c, 204, nil)
}
