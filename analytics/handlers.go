package analytics

import (
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/aggregate"
	"hanzo.io/models/analytics"
	"hanzo.io/models/organization"
	"hanzo.io/models/types/client"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"

	. "hanzo.io/util/aggregate/tasks"
	. "hanzo.io/util/analytics/tasks"
)

func create(c *gin.Context) {
	receivedTime := time.Now()

	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)

	id := c.Params.ByName("organizationid")
	org := organization.New(db)
	if err := org.GetById(id); err != nil {
		http.Fail(c, 400, "Failed to get organization", err)
		return
	}

	db = datastore.New(org.Namespaced(ctx))

	var events []*analytics.AnalyticsEvent

	if err := json.Decode(c.Request.Body, &events); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	if len(events) > 0 {
		mostRecentTime := events[len(events)-1].Timestamp
		for _, event := range events {
			offset := event.Timestamp.Sub(mostRecentTime)

			event.Db = db
			event.Entity = event
			event.RequestMetadata = client.New(c)
			event.CalculatedTimestamp = receivedTime.Add(offset)
			if err := event.Put(); err != nil {
				http.Fail(c, 500, "Failed to create event", err)
				return
			}

			UpsertAggregate(ctx, org.Name, event.Name, "AnalyticsEvent", event.CalculatedTimestamp, aggregate.Hourly, 1, nil)
			UpdateFunnels(ctx, org.Name, event.Id())
		}
	}
	http.Render(c, 204, nil)
}
