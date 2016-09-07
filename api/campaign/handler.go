package xd

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/models/campaign"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/rest"
	"crowdstart.com/util/router"
)

type ProgressRes struct {
	Progress float64 `json:"progress"`
}

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := rest.New(campaign.Campaign{})

	api.GET("/:campaignid/progress", func(c *gin.Context) {
		// hardcoded for KANOA
		now := time.Now()
		startDate := time.Date(2016, time.September, 6, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2017, time.March, 15, 0, 0, 0, 0, time.UTC)
		daysTotal := endDate.Sub(startDate).Hours() / 24
		days := now.Sub(startDate).Hours() / 24
		daysComplete := days / daysTotal

		startPct := 3.0

		progress := math.Min(startPct+((100.0-startPct)*daysComplete), 99.9)
		// Go has no math.Round, sadly
		f, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", progress), 64)
		http.Render(c, 200, ProgressRes{f})
	})

	api.Route(router, args...)
}
