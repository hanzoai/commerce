package platform

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/fee"
	"crowdstart.com/models/multi"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/transfer"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/log"
)

func Payout(db *datastore.Datastore) error {
	ctx := db.Context
	log.Debug("Fetching all organizations", ctx)
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(db).GetAll(&orgs); err != nil {
		log.Error("Failed to fetch organizations", ctx)
		return err
	}
	return nil
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
