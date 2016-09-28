package cart

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/models/cart"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/rest"
	"crowdstart.com/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
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
