package cart

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/cart"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/rest"
)

func Route(router zip.Router, args ...zip.Handler) {
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)
	namespaced := middleware.Namespace()

	api := rest.New(cart.Cart{})

	api.Create = create(api)
	api.Update = update(api)
	api.Patch = patch(api)

	api.POST("/:cartid/set", publishedRequired, namespaced, Set)
	api.POST("/:cartid/discard", publishedRequired, namespaced, Discard)

	api.Route(router, args...)
}
