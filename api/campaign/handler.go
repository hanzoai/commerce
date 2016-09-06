package xd

import (
	"math"
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/models/campaign"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/rest"
	"crowdstart.com/util/router"
)

type ProgressRes struct {
	Progress interface{} `json:"progress"`
}

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := rest.New(campaign.Campaign{})

	api.GET("/:campaignid/progress", func(c *gin.Context) {
		// hardcoded for KANOA
		now := time.Now()
		startDate := time.Date(2016, time.May, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2017, time.March, 15, 0, 0, 0, 0, time.UTC)
		daysTotal := endDate.Sub(startDate).Hours() / 24
		days := now.Sub(startDate).Hours() / 24
		daysComplete := days / daysTotal

		startPct := 3.0

		progress := math.Min(startPct+((100.0-startPct)*daysComplete), 99.9)
		http.Render(c, 200, ProgressRes{progress})
	})

	api.Route(router, args...)
}
