package affiliate_transfer

import (
	"fmt"
	"time"

	"appengine"
	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/affiliate"
	"crowdstart.com/models/fee"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/transfer"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/log"
	"github.com/gin-gonic/gin"
)

func CutoffForAffiliate(aff affiliate.Affiliate, now time.Time) time.Time {
	year, month, day := now.UTC().Date()
	ret := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	ret = ret.AddDate(0, 0, -aff.Period)
	return ret
}

func fetchFeesForAffiliate(ds *datastore.Datastore, aff affiliate.Affiliate, now time.Time) (map[currency.Type][]fee.Fee, error) {
	affId := aff.Id()
	cutoff := CutoffForAffiliate(aff, now)
	rawfees := make([]fee.Fee, 0, 0)
	_, err := ds.Query(fee.Fee{}.Kind()).
		Filter("AffiliateId =", affId).
		Filter("TransferId =", "").
		Filter("CreatedAt <", cutoff).
		GetAll(&rawfees)
	if err != nil {
		return nil, err
	}
	fees := make(map[currency.Type][]fee.Fee)
	for _, fee := range rawfees {
		cfees := fees[fee.Currency]
		cfees = append(cfees, fee)
		fees[fee.Currency] = cfees
	}
	return fees, nil
}

func fetchFeesForPlatform(ds *datastore.Datastore, now time.Time) (map[currency.Type][]fee.Fee, error) {
	year, month, day := now.UTC().Date()
	cutoff := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	cutoff = cutoff.AddDate(0, 0, -1)
	rawfees := make([]fee.Fee, 0, 0)
	_, err := ds.Query(fee.Fee{}.Kind()).
		Filter("Type =", fee.Platform).
		Filter("TransferId =", "").
		Filter("CreatedAt <", cutoff).
		GetAll(&rawfees)
	if err != nil {
		return nil, err
	}
	fees := make(map[currency.Type][]fee.Fee)
	for _, fee := range rawfees {
		cfees := fees[fee.Currency]
		cfees = append(cfees, fee)
		fees[fee.Currency] = cfees
	}
	return fees, nil
}

func createTransfer(ds *datastore.Datastore, currency currency.Type, destination string, destinationType string) *transfer.Transfer {
	var tr transfer.Transfer
	tr.Defaults()
	tr.Currency = currency
	tr.DestinationType = destinationType
	tr.Destination = destination
	tr.MustPut()
	return &tr
}

func associateFeesToTransfers(ds *datastore.Datastore, fees map[currency.Type][]fee.Fee, destination string, destinationType string) (map[currency.Type]*transfer.Transfer, error) {
	if destination == "" {
		return nil, fmt.Errorf("associateFeesToTransfers: invalid invocation.  empty destination.")
	}
	if destinationType == "" {
		return nil, fmt.Errorf("associateFeesToTransfers: invalid invocation.  empty destinationType.")
	}
	ret := make(map[currency.Type]*transfer.Transfer)
	for currency, cfees := range fees {
		for i, fee := range cfees {
			if fee.Currency != currency {
				return nil, fmt.Errorf("associateFeesToTransfers: should be impossible: currency mismatch for fee %v", fee.Id())
			}
			tr, ok := ret[currency]
			if !ok {
				tr = createTransfer(ds, currency, destination, destinationType)
			}
			fee.TransferId = tr.Id()
			tr.Amount = tr.Amount + fee.Amount
			txfn := func(ctx appengine.Context) error {
				ds := datastore.New(ctx)
				_, err := ds.Put(fee.Id(), fee)
				if err != nil {
					return err
				}
				_, err = ds.Put(tr.Id(), tr)
				if err != nil {
					return err
				}
				return nil
			}
			txopts := &aeds.TransactionOptions{XG: true}
			err := ds.RunInTransaction(txfn, txopts)
			if err != nil {
				return nil, err
			}
			ret[currency] = tr
			cfees[i] = fee
		}
		fees[currency] = cfees
	}
	return ret, nil
}

func sendTransferToStripe(st *stripe.Client, tr *transfer.Transfer) {
	_, err := st.Transfer(tr)
	if err != nil {
		panic(err)
	}
	tr.MustPut()
}

func processAffiliateFees(ds *datastore.Datastore, aff affiliate.Affiliate, now time.Time) {
	fees, err := fetchFeesForAffiliate(ds, aff, now)
	if err != nil {
		log.Warn(err)
	}
	trs, err := associateFeesToTransfers(ds, fees, aff.Stripe.UserId, string(fee.Affiliate))
	if err != nil {
		panic(err)
	}
	st := makeStripeApi(ds)
	for _, tr := range trs {
		sendTransferToStripe(st, tr)
		_, err := st.Transfer(tr)
		if err != nil {
			panic(err)
		}
	}
}

func processPlatformFees(ds *datastore.Datastore, now time.Time) {
	fees, err := fetchFeesForPlatform(ds, now)
	if err != nil {
		log.Warn(err)
	}
	trs, err := associateFeesToTransfers(ds, fees, "default_for_currency", string(fee.Platform))
	if err != nil {
		panic(err)
	}
	st := makeStripeApi(ds)
	for _, tr := range trs {
		sendTransferToStripe(st, tr)
		_, err := st.Transfer(tr)
		if err != nil {
			panic(err)
		}
	}
}

type retryError struct {
	transferKey string
	err         error
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
		trs := make([]transfer.Transfer, 0, len(keys))
		for _, key_ := range keys {
			key := key_.Encode()
			var tr transfer.Transfer
			err := ds.Get(key, &tr)
			if err != nil {
				errs = append(errs, retryError{key, err})
			} else {
				trs = append(trs, tr)
			}
		}
		if len(errs) > 0 {
			log.Warn("failures while fetching incomplete transfers: %v", errs)
		}
		st := makeStripeApi(ds)
		for _, tr := range trs {
			sendTransferToStripe(st, &tr)
		}
	}
}

func makeStripeApi(ds *datastore.Datastore) *stripe.Client {
	org := organization.New(ds)
	return stripe.New(ds.Context, org.Stripe.AccessToken)
}

func fetchAllAffiliates(ds *datastore.Datastore) []affiliate.Affiliate {
	affiliates := make([]affiliate.Affiliate, 0, 0)
	_, err := ds.Query(affiliate.Affiliate{}.Kind()).GetAll(&affiliates)
	if err != nil {
		log.Error("failed to fetch affiliates: %v", err)
		panic(err)
	}
	return affiliates
}

func Run(c *gin.Context) {
	panic("XXXih: work in progress")
	ds := datastore.New(c)
	retryIncompleteTransfers(ds)
	now := time.Now()
	affiliates := fetchAllAffiliates(ds)
	for _, aff := range(affiliates) {
		processAffiliateFees(ds, aff, now)
	}
}
