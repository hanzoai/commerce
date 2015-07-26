package analytics

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/analytics"
	"crowdstart.com/models/types/client"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"

	. "crowdstart.com/util/analytics"
)

func create(c *gin.Context) {
	receivedTime := time.Now()

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespace(c))
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
			UpdateFunnels.Call(db.Context, org, event)
		}
	}
	http.Render(c, 204, nil)
}
