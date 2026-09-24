package account

import (
	"net/http"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
)

func Route(router *zip.Group, args ...zip.Handler) {
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)
	accountRequired := middleware.AccountRequired()
	namespaced := middleware.Namespace()

	api := router.Group("account")
	api.Use(publishedRequired)

	api.Raw(http.MethodGet, "", accountRequired, namespaced, get)
	api.Raw(http.MethodPut, "", accountRequired, namespaced, update)
	api.Raw(http.MethodPatch, "", accountRequired, namespaced, patch)

	api.Raw(http.MethodGet, "/order/:orderid", accountRequired, namespaced, getOrder)
	api.Raw(http.MethodPatch, "/order/:orderid", accountRequired, namespaced, patchOrder)
	api.Raw(http.MethodPost, "/withdraw", accountRequired, namespaced, withdraw)
	api.Raw(http.MethodPost, "/paymentmethod/:paymentmethodtype", accountRequired, namespaced, createPaymentMethod)

	api.Raw(http.MethodGet, "/exists/:emailorusername", namespaced, exists)

	api.Raw(http.MethodPost, "/login", namespaced, login)

	api.Raw(http.MethodPost, "/create", namespaced, create)
	api.Raw(http.MethodPost, "/enable/:tokenid", namespaced, enable)

	api.Raw(http.MethodPost, "/reset", namespaced, reset)
	api.Raw(http.MethodPost, "/confirm/:tokenid", namespaced, confirm)
}
