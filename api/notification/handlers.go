package notification

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	notificationModel "github.com/hanzoai/commerce/models/notification"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rest"
)

func Route(router zip.Router, args ...zip.Handler) {
	namespaced := middleware.Namespace()

	api := rest.New(notificationModel.Notification{})
	api.POST("/:notificationid/resend", namespaced, Resend)
	api.Route(router, args...)
}

// Resend resets a notification's status to pending so it can be re-delivered.
func Resend(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	id := c.Param("notificationid")

	n := notificationModel.New(db)
	if err := n.GetById(id); err != nil {
		return http.Fail(c, 404, "No notification found with id: "+id, err)
	}

	// Reset status to pending for re-delivery
	n.Status = notificationModel.Pending
	n.ExternalId = ""

	if err := n.Update(); err != nil {
		return http.Fail(c, 500, "Failed to resend notification", err)
	}

	return http.Render(c, 200, n)
}
