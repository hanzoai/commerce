package review

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/review"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/rest"
	"crowdstart.com/util/router"
)

var forced404 = func(c *gin.Context) {
	http.Fail(c, 404, "Not found", nil)
}

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := rest.New(review.Review{})

	api.Update = forced404
	api.Patch = forced404

	api.Route(router, args...)
}
