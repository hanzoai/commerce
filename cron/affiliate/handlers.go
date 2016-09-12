package affiliate

import (
	"time"

	"crowdstart.com/models/affiliate"
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
