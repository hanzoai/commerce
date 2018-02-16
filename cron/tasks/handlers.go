package tasks

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/cron/payout/affiliate"
	"hanzo.io/cron/payout/partner"
	"hanzo.io/cron/payout/platform"
	"hanzo.io/middleware"
	"hanzo.io/util/task"
)

// Register tasks
func init() {
	task.New("payout-affiliate", func(c *context.Context) {
		ctx := middleware.GetAppEngine(c)
		affiliate.Payout(ctx)
	})

	task.New("payout-partner", func(c *context.Context) {
		ctx := middleware.GetAppEngine(c)
		partner.Payout(ctx)
	})

	task.New("payout-platform", func(c *context.Context) {
		ctx := middleware.GetAppEngine(c)
		platform.Payout(ctx)
	})
}
