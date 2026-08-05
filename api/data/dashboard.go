package data

import (
	"strconv"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/counter"
	"github.com/hanzoai/commerce/util/json/http"
)

func dashboard(c *zip.Ctx) error {
	period := counter.Period(c.Param("period"))
	year, _ := strconv.Atoi(c.Param("year"))
	month, _ := strconv.Atoi(c.Param("month"))
	day, _ := strconv.Atoi(c.Param("day"))
	// tzOffset, _ := strconv.Atoi(c.Param("tzOffset"))

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

	data, err := counter.GetDashboardData(org.Context(), period, date, -8*3600, org)
	if err != nil {
		return http.Fail(c, 500, "Failed to load data", err)
	}
	return http.Render(c, 200, data)
}
