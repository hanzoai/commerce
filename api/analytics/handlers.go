package analytics

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	group := router.Group("analytics")
	group.GET("/:organizationid/js", js)
}
