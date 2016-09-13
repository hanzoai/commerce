package tasks

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/cron/affiliate"
	"crowdstart.com/cron/platform"
	"crowdstart.com/middleware"
	"crowdstart.com/util/task"
)

// Register tasks
func init() {
	task.New("payout-affiliate", func(c *gin.Context) {
		ctx := middleware.GetAppEngine(c)
		affiliate.Payout(ctx)
	})

	task.New("payout-platform", func(c *gin.Context) {
		ctx := middleware.GetAppEngine(c)
		platform.Payout(ctx)
	})
}
