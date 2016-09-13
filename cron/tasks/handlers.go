package tasks

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/cron/affiliate"
	"crowdstart.com/cron/platform"
	"crowdstart.com/datastore"
	"crowdstart.com/util/task"
)

// Register tasks
func init() {
	task.New("payout-affiliate", func(c *gin.Context) {
		db := datastore.New(c)
		affiliate.Payout(db)
	})

	task.New("payout-platform", func(c *gin.Context) {
		db := datastore.New(c)
		platform.Payout(db)
	})
}
