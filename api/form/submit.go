package form

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/form"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/submission"
	"github.com/hanzoai/commerce/models/types/client"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
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
