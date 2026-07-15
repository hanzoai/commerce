package form

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/form"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/submission"
	"github.com/hanzoai/commerce/models/types/client"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
)

func submit(c *zip.Ctx, db *datastore.Datastore, org *organization.Organization, f *form.Form) error {
	ctx := db.Context

	// Make sure Subscriber is created with the right context
	s := submission.New(db)

	// Decode response body for subscriber
	if err := json.DecodeBytes(c.Body(), s); err != nil {
		return http.Fail(c, 400, "Failed decode request body", err)
	}

	// Store metadata about client
	s.Client = client.New(c)

	// Forward submission (if enabled)
	forward(ctx, org, f, s)

	// Success!
	return http.Render(c, 200, s)
}
