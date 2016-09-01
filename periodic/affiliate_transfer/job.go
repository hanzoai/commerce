package affiliate_transfer

import (
	"time"

	"appengine"
	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/datastore/parallel"
	"crowdstart.com/models/affiliate"
	"crowdstart.com/models/fee"
	"crowdstart.com/models/transfer"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/models/organization"
	"crowdstart.com/util/log"
	"github.com/gin-gonic/gin"
)

func cutoffForAffiliate(aff affiliate.Affiliate, now time.Time) {
	year, month, day := t.UTC().Date()
	ret := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	ret.AddDate(0, 0, -aff.Period)
	return ret
}

func fetchFeesForAffiliate(ds *datastore.Datastore, aff affiliate.Affiliate, now time.Time) ([]fee.Fee, error) {
	affId := aff.Id()
	cutoff := cutoffForAffiliate(aff, now)
	fees := make([]fee.Fee, 0, 0)
	_, err := ds.Query(fee.Fee{}.Kind()).
		Filter("AffiliateId =", affId).
		Filter("TransferId =", "").
		Filter("CreatedAt <", cutoff).
		GetAll(&fees)
	if err != nil {
		return nil, err
	}
	return fees, nil
}

func createTransfer(ds *datastore.Datastore) transfer.Transfer {
	var tr transfer.Transfer
	tr.Defaults()
	tr.MustPut()
	return tr
}

func associateFeesToTransfer(ds *datastore.Datastore, fees []fee.Fee, tr *transfer.Transfer) error {
	for i, fee := range(fees) {
		tr_ := *tr
		fee.TransferId = tr.Id()
		// XXXih: need to handle non-USD
		tr_.Amount = tr_.Amount + fee.Amount
		txfn := func (ctx appengine.Context) error {
			ds := datastore.New(ctx)
			_, err := ds.Put(fee.Id(), fee)
			if err != nil {
				return err
			}
			_, err = ds.Put(tr_.Id(), tr_)
			if err != nil {
				return err
			}
			return nil
		}
		txopts := &aeds.TransactionOptions{XG: true}
		err := ds.RunInTransaction(txfn, txopts)
		if err != nil {
			return err
		}
		*tr = tr_
		fees[i] = fee
	}
	return nil
}

func sendTransferToStripe(ds *datastore.Datastore, tr *transfer.Transfer) {
	st := makeStripeApi(ds)
	_, err := st.Transfer(tr)
	if err != nil {
		panic(err)
	}
}

func processAffiliateFees(ds *datastore.Datastore, aff affiliate.Affiliate, now time.Time) {
	fees, err := fetchFeesForAffiliate(ds, aff, now)
	if err != nil {
		log.Warn(err)
	}
	tr := createTransfer(ds)
	err = associateFeesToTransfer(ds, fees, &tr)
	if err != nil {
		panic(err)
	}
	sendTransferToStripe(ds, &tr)
}

type retryError struct {
	transferKey string
	err error
}

func retryIncompleteTransfers(ds *datastore.Datastore) {
	errs := make([]retryError, 0, 0)
	keys, err := ds.Query(transfer.Transfer{}.Kind()).
		Filter("Status =", "Initializing").
		Filter("Amount >", 0).
		KeysOnly().
		GetAll(nil)
	if err != nil {
		log.Error("failed to fetch keys for incomplete transfers: %v", err)
	} else {
		transfers := make([]transfer.Transfer, 0, len(keys))
		for _, key_ := range(keys) {
			key := key_.Encode()
			var tr transfer.Transfer
			err := ds.Get(key, &tr)
			if err != nil {
				errs = append(errs, retryError{key, err})
			} else {
				transfers = append(transfers, tr)
			}
		}
		if len(errs) > 0 {
			log.Warn("failures while fetching incomplete transfers: %v", errs)
		}
		for _, tr := range(transfers) {
			sendTransferToStripe(ds, &tr)
		}
	}
}

func makeStripeApi(ds *datastore.Datastore) *stripe.Client {
	org := organization.New(ds)
	return stripe.New(ds.Context, org.Stripe.AccessToken)
}

var pfn = parallel.New("periodic-affiliate_transfer-task", processAffiliateFees)

// XXXih: the typical lifecycle of a Transfer is as follows:
// 1. a Transfer is created and stored to datastore; this produces a unique ID
// 2. the aforementioned unique ID is then used as an "idempotency tag" in all
//    associated requests to our payment processor
// 3. ...
func Run(c *gin.Context) {
	panic("XXXih: work in progress")
	ds := datastore.New(c)
	retryIncompleteTransfers(ds)
	now := time.Now()
	pfn.Run(c, 100, now)
}
