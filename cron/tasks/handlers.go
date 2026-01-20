package tasks

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/cron/payout/affiliate"
	"github.com/hanzoai/commerce/cron/payout/partner"
	"github.com/hanzoai/commerce/cron/payout/platform"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/task"
)

// Register tasks
func init() {
	task.New("payout-affiliate", func(c *gin.Context) {
		ctx := middleware.GetAppEngine(c)
		affiliate.Payout(ctx)
	})

	task.New("payout-partner", func(c *gin.Context) {
		ctx := middleware.GetAppEngine(c)
		partner.Payout(ctx)
	})

	task.New("payout-platform", func(c *gin.Context) {
		ctx := middleware.GetAppEngine(c)
		platform.Payout(ctx)
	})
}
