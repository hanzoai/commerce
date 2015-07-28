package analytics

import (
	"os"
	"strings"
	"time"

	"appengine"

	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/aggregate"
	"crowdstart.com/models/analytics"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/types/client"
	"crowdstart.com/util/fs"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"

	. "crowdstart.com/util/aggregate/tasks"
	. "crowdstart.com/util/analytics/tasks"
)

func create(c *gin.Context) {
	receivedTime := time.Now()

	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)

	id := c.Params.ByName("orgid")
	org := organization.New(db)
	if err := org.Get(id); err != nil {
		http.Fail(c, 400, "Failed to get organization", err)
		return
	}

	db = datastore.New(org.Namespace(ctx))

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

var jsTemplate string

func js(c *gin.Context) {
	db := datastore.New(c)

	id := c.Params.ByName("orgid")
	org := organization.New(db)
	if err := org.Get(id); err != nil {
		http.Fail(c, 400, "Failed to get organization", err)
		return
	}

	if jsTemplate == "" {
		var cwd, _ = os.Getwd()
		jsTemplate = string(fs.ReadFile(cwd + "/js/native.js"))
	}

	// Endpoint for subscription
	endpoint := config.UrlFor("analytics", "/"+org.Id())
	if appengine.IsDevAppServer() {
		endpoint = "http://localhost:8080" + endpoint
	} else {
		endpoint = "https:" + endpoint
	}

	c.Writer.Header().Add("Content-Type", "application/javascript")

	script := strings.Replace(jsTemplate, "%%%%%url%%%%%", config.UrlFor("analytics", "/"+org.Id()+"/"), -1)

	c.String(200, script)
}
