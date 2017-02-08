package form

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/form"
	"hanzo.io/models/organization"
	"hanzo.io/models/submission"
	"hanzo.io/models/types/client"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
)

func submit(c *gin.Context, db *datastore.Datastore, org *organization.Organization, f *form.Form) {
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
	forward(ctx, org, f, s)

	// Success!
	http.Render(c, 200, s)
}
