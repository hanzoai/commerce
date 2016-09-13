package platform

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/fee"
	"crowdstart.com/models/multi"
	"crowdstart.com/models/transfer"
	"crowdstart.com/thirdparty/stripe"
	"github.com/gin-gonic/gin"
)

func Payout(c *gin.Context) {
}

func fetchFees(db *datastore.Datastore) ([]*fee.Fee, error) {
	fees := make([]*fee.Fee, 0)
	if _, err := fee.Query(db).Filter("TransferId=", "").GetAll(&fees); err != nil {
		return nil, err
	}
	return fees, nil
}

func transferFee(db *datastore.Datastore, sc *stripe.Client, fe *fee.Fee) error {
	tr := transfer.New(db)
	fe.TransferId = tr.Id()
	fe.Status = fee.Paid
	if _, err := sc.Transfer(tr); err != nil {
		return err
	}
	models := []interface{}{tr, fe}
	return multi.Update(models)
}
