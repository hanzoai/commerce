package tasks

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/cron/payout/affiliate"
	"github.com/hanzoai/commerce/cron/payout/partner"
	"github.com/hanzoai/commerce/cron/payout/platform"
	"github.com/hanzoai/commerce/util/task"
)

// Register tasks
func init() {
	task.New("payout-affiliate", func(c *zip.Ctx) error {
		ctx := c.Context()
		return affiliate.Payout(ctx)
	})

	task.New("payout-partner", func(c *zip.Ctx) error {
		ctx := c.Context()
		return partner.Payout(ctx)
	})

	task.New("payout-platform", func(c *zip.Ctx) error {
		ctx := c.Context()
		return platform.Payout(ctx)
	})
}
