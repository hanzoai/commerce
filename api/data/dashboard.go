package data

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/thirdparty/redis"
	"crowdstart.com/util/json/http"
)

func dashboard(c *gin.Context) {
	org := middleware.GetOrganization(c)

	data, err := redis.GetDashboardData(org.Db.Context, redis.Weekly, time.Now(), org)
	if err != nil {
		http.Fail(c, 500, "Failed to load data", err)
	} else {
		http.Render(c, 200, data)
	}
}
