package account

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
)

func Route(router zip.Router, args ...zip.Handler) {
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)
	accountRequired := middleware.AccountRequired()
	namespaced := middleware.Namespace()

	api := router.Group("account")
	api.Use(publishedRequired)

	api.Get("", accountRequired, namespaced, get)
	api.Put("", accountRequired, namespaced, update)
	api.Patch("", accountRequired, namespaced, patch)

	api.Get("/order/:orderid", accountRequired, namespaced, getOrder)
	api.Patch("/order/:orderid", accountRequired, namespaced, patchOrder)
	api.Post("/withdraw", accountRequired, namespaced, withdraw)
	api.Post("/paymentmethod/:paymentmethodtype", accountRequired, namespaced, createPaymentMethod)

	api.Get("/exists/:emailorusername", namespaced, exists)

	api.Post("/login", namespaced, login)

	api.Post("/create", namespaced, create)
	api.Post("/enable/:tokenid", namespaced, enable)

	api.Post("/reset", namespaced, reset)
	api.Post("/confirm/:tokenid", namespaced, confirm)
}
