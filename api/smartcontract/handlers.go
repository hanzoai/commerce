package smartcontract

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/models/smartcontract"
	"hanzo.io/util/rest"
	"hanzo.io/util/router"
)

func Call(c *gin.Context) {
}

func Route(router router.Router, args ...gin.HandlerFunc) {
	namespaced := middleware.Namespace()
	tokenRequired := middleware.TokenRequired()

	api := rest.New(smartcontract.SmartContract{})
	api.POST("/:smartcontractid/call", tokenRequired, namespaced, Call)
	api.Route(router, args...)
}
