package data

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/counter"
	"hanzo.io/util/json/http"
	"hanzo.io/util/log"
)

func dashboard(c *gin.Context) {
	period := counter.Period(c.Params.ByName("period"))
	year, _ := strconv.Atoi(c.Params.ByName("year"))
	month, _ := strconv.Atoi(c.Params.ByName("month"))
	day, _ := strconv.Atoi(c.Params.ByName("day"))
	// tzOffset, _ := strconv.Atoi(c.Params.ByName("tzOffset"))

	switch period {
	case counter.Yearly:
	case counter.Weekly:
	case counter.Monthly:
	case counter.Daily:
	default:
		period = counter.Weekly
	}

	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)

	log.Warn("date %v\n period %v", date, period)

	org := middleware.GetOrganization(c)

	data, err := counter.GetDashboardData(org.Db.Context, period, date, -8*3600, org)
	if err != nil {
		http.Fail(c, 500, "Failed to load data", err)
	} else {
		http.Render(c, 200, data)
	}
}
