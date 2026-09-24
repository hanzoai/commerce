package transaction

import (
	"net/http"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
)

func Route(router *zip.Group, args ...zip.Handler) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := router.Group("transaction")

	// Auth and Capture Flow (Two-step Payment)
	api.Raw(http.MethodPost, "", adminRequired, Create)
	api.Raw(http.MethodGet, "/:kind/:id", adminRequired, List)
	api.Raw(http.MethodPost, "/hold", adminRequired, CreateHold)
	api.Raw(http.MethodDelete, "/hold/:id", adminRequired, RemoveHold)
}
