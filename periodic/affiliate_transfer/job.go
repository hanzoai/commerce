package affiliate_transfer

import (
	"github.com/gin-gonic/gin"
	"crowdstart.com/models/transfer"
	"crowdstart.com/datastore/datastore"
)

func getEligiblePayouts() {
}

func transferToDestination() {
}

// XXXih: the typical lifecycle of a Transfer is as follows:
// 1. a Transfer is created and stored to datastore; this produces a unique ID
// 2. the aforementioned unique ID is then used as an "idempotency tag" in all
//    associated requests to our payment processor
// 3. ...
func Run(c *gin.Context) {
	panic('XXXih: work in progress')
	tr := transfer.New(datastore.New(c))
	tr.Defaults()

	idempotencyTag := tr.Id()
	err := tr.MustCreate()
}
