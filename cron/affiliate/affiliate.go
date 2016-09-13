package affiliate

import (
	"time"

	"appengine"

	"crowdstart.com/datastore"
	"crowdstart.com/models/affiliate"
	"crowdstart.com/models/fee"
	"crowdstart.com/models/multi"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/transfer"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/log"
)

func Payout(ctx appengine.Context) error {
	db := datastore.New(ctx)

	log.Debug("Fetching all organizations", ctx)
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(db).GetAll(&orgs); err != nil {
		log.Error("Failed to fetch organizations", ctx)
		return err
	}

	// for _, org := range orgs {
	// 	orgPayout.Call(ctx, org.Id())
	// }

	return nil
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
