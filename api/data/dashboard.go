package data

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/thirdparty/redis"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"
)

func dashboard(c *gin.Context) {
	period := redis.Period(c.Params.ByName("period"))
	year, _ := strconv.Atoi(c.Params.ByName("year"))
	month, _ := strconv.Atoi(c.Params.ByName("month"))
	day, _ := strconv.Atoi(c.Params.ByName("day"))

	switch period {
	case redis.Yearly:
	case redis.Weekly:
	case redis.Monthly:
	case redis.Daily:
	default:
		period = redis.Weekly
	}

	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)

	log.Warn("date %v\n period %v", date, period)

	org := middleware.GetOrganization(c)

	data, err := redis.GetDashboardData(org.Db.Context, period, date, org)
	if err != nil {
		http.Fail(c, 500, "Failed to load data", err)
	} else {
		http.Render(c, 200, data)
	}
}
