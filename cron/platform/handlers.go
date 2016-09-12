package platform

import (
	"time"

	"crowdstart.com/datastore"
	"crowdstart.com/models/fee"
	"crowdstart.com/models/multi"
	"crowdstart.com/models/transfer"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/thirdparty/stripe"
	"github.com/gin-gonic/gin"
)

type feeMap map[currency.Type][]fee.Fee

func Payout(c *gin.Context) {

}

func fetchFeesForPlatform(db *datastore.Datastore) (feeMap, error) {
	year, month, day := time.Now().UTC().Date()
	cutoff := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	cutoff = cutoff.AddDate(0, 0, -1)
	rawfees := make([]fee.Fee, 0, 0)
	_, err := db.Query(fee.Fee{}.Kind()).
		Filter("Type =", fee.Platform).
		Filter("TransferId =", "").
		Filter("CreatedAt <", cutoff).
		GetAll(&rawfees)
	if err != nil {
		return nil, err
	}
	fees := make(feeMap)
	for _, fee := range rawfees {
		cfees := fees[fee.Currency]
		cfees = append(cfees, fee)
		fees[fee.Currency] = cfees
	}
	return fees, nil
}

func sendTransferToStripe(st *stripe.Client, tr *transfer.Transfer) error {
	_, err := st.Transfer(tr)
	if err != nil {
		return err
	}
	tr.MustPut()
	return nil
}

func markFeesPaid(fees []*fee.Fee) error {
	for _, fe := range fees {
		fe.Status = fee.Paid
	}
	err := multi.Update(fees)
	if err != nil {
		return err
	}
	return nil
}
