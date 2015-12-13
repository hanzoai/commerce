package form

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mailinglist"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/submission"
	"crowdstart.com/models/types/client"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
)

func submit(c *gin.Context, db *datastore.Datastore, org *organization.Organization, ml *mailinglist.MailingList) {
	ctx := db.Context

	// Make sure Subscriber is created with the right context
	s := submission.New(db)

	// Decode response body for subscriber
	if err := json.Decode(c.Request.Body, s); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Store metadata about client
	s.Client = client.New(c)

	// Forward submission (if enabled)
	forward(ctx, org, ml, s)

	// Success!
	http.Render(c, 200, s)
}
