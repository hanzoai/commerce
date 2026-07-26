// Package currency is the HTTP surface for the currency reference table: admin
// CRUD (generic REST) plus a public GET list. Currencies are a GLOBAL reference
// (default namespace, like token/user/organization) — one shared set the
// store/settings and product/price pickers read, replacing a hardcoded array.
//
// Two audiences:
//   - PUBLIC read: GET /v1/commerce/currencies returns the whole list, no auth
//     (it is presentation/reference data). Wired on the commerce public group.
//   - ADMIN write: create/update/delete mutate the reference set and are gated by
//     the /v1 bundle's adminRequired. Wired in api/api.go.
package currency

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	currencyModel "github.com/hanzoai/commerce/models/currency"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rest"
)

// Route wires the admin CRUD on the /v1 bundle. Currencies use the DEFAULT
// namespace (a global reference set), so DefaultNamespace is set — the generic
// REST scaffold then serves /currency (list/get/create/update/delete).
func Route(router zip.Router, args ...zip.Handler) {
	api := rest.New(currencyModel.Currency{})
	api.DefaultNamespace = true
	api.Route(router, args...)
}

// PublicRoute wires the unauthenticated list projection. Mount on the commerce
// public group so it serves GET /v1/commerce/currencies.
func PublicRoute(r zip.Router) {
	r.Get("/currencies", List)
}

// List returns every reference currency (default namespace). Public + cacheable.
func List(c *zip.Ctx) error {
	db := datastore.New(c.Context())
	rows := make([]*currencyModel.Currency, 0, 32)
	if _, err := currencyModel.Query(db).GetAll(&rows); err != nil {
		return http.Fail(c, 500, "Failed to list currencies", err)
	}
	return http.Render(c, 200, rows)
}
