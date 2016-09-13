package affiliate

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/util/task"
)

// Handler to trigger payout
func Payout(c *gin.Context) {
	db := datastore.New(c)
	payout(db)
}

// Register task
var _ = task.New("payout-affiliate", Payout)
