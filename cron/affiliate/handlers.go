package affiliate

import (
	"time"

	"crowdstart.com/models/affiliate"
	"crowdstart.com/models/fee"
	"crowdstart.com/models/multi"
	"crowdstart.com/models/transfer"
	"crowdstart.com/thirdparty/stripe"
	"github.com/gin-gonic/gin"
)

func Payout(c *gin.Context) {

}

func isEligibleForPayout(aff *affiliate.Affiliate) bool {
	year, month, day := time.Now().UTC().Date()
	dateToday := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	dateToPay := aff.LastPaid.AddDate(0, 0, aff.Period)
	return (dateToday.After(dateToPay) || dateToday == dateToPay)
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
