package stripe_payout

import (
	"github.com/gin-gonic/gin"
)

func getEligiblePayouts() {
}

func transferToDestination() {
}

// XXXih: TODO:
// 1. synthesize and store an idempotency key for each pending transfer
// 2. track (and store) the current state of each transfer (e.g. data TransferState = Pending IdempotencyKey | Done)
// 3. ???
func Run(c *gin.Context) {
}
