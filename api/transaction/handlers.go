package transaction

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
)

func Route(router zip.Router, args ...zip.Handler) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := router.Group("transaction")

	// Auth and Capture Flow (Two-step Payment)
	api.Post("", adminRequired, Create)
	api.Get("/:kind/:id", adminRequired, List)
	api.Post("/hold", adminRequired, CreateHold)
	api.Delete("/hold/:id", adminRequired, RemoveHold)
}
