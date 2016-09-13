package affiliate_transfer

import (
	"fmt"
	"time"

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

type feeMap map[currency.Type][]fee.Fee
type transferMap map[currency.Type]*transfer.Transfer

func CutoffForAffiliate(aff affiliate.Affiliate, now time.Time) time.Time {
	year, month, day := now.UTC().Date()
	ret := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	ret = ret.AddDate(0, 0, -aff.Period)
	return ret
}

func fetchFeesForAffiliate(db *datastore.Datastore, aff affiliate.Affiliate, now time.Time) (feeMap, error) {
	affId := aff.Id()
	cutoff := CutoffForAffiliate(aff, now)
	rawfees := make([]fee.Fee, 0, 0)

	_, err := db.Query(fee.Fee{}.Kind()).
		Filter("AffiliateId =", affId).
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

func fetchFeesForPlatform(db *datastore.Datastore, now time.Time) (feeMap, error) {
	year, month, day := now.UTC().Date()
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

func createTransfer(db *datastore.Datastore, currency currency.Type, destination string, destinationType string) *transfer.Transfer {
	var tr transfer.Transfer
	tr.Defaults()
	tr.Currency = currency
	tr.DestinationType = destinationType
	tr.Destination = destination
	tr.MustPut()
	return &tr
}

func associateFeesToTransfers(db *datastore.Datastore, fees feeMap, destination string, destinationType string) (transferMap, error) {
	if destination == "" {
		return nil, fmt.Errorf("associateFeesToTransfers: invalid invocation.  empty destination.")
	}
	if destinationType == "" {
		return nil, fmt.Errorf("associateFeesToTransfers: invalid invocation.  empty destinationType.")
	}
	ret := make(transferMap)
	for currency, cfees := range fees {
		for i, fee := range cfees {
			if fee.Currency != currency {
				return nil, fmt.Errorf("associateFeesToTransfers: should be impossible: currency mismatch for fee %v", fee.Id())
			}
			tr, ok := ret[currency]
			if !ok {
				tr = createTransfer(db, currency, destination, destinationType)
			}
			fee.TransferId = tr.Id()
			tr.Amount = tr.Amount + fee.Amount
			txfn := func(db *datastore.Datastore) error {
				_, err := db.Put(fee.Id(), fee)
				if err != nil {
					return err
				}
				_, err = db.Put(tr.Id(), tr)
				if err != nil {
					return err
				}
				return nil
			}
			txopts := &datastore.TransactionOptions{XG: true}
			err := db.RunInTransaction(txfn, txopts)
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

func processAffiliateFees(db *datastore.Datastore, aff affiliate.Affiliate, now time.Time) {
	fees, err := fetchFeesForAffiliate(db, aff, now)
	if err != nil {
		log.Warn(err)
	}
	trs, err := associateFeesToTransfers(db, fees, aff.Stripe.UserId, string(fee.Affiliate))
	if err != nil {
		panic(err)
	}
	st := makeStripeApi(db)
	for _, tr := range trs {
		sendTransferToStripe(st, tr)
		_, err := st.Transfer(tr)
		if err != nil {
			panic(err)
		}
	}
}

func processPlatformFees(db *datastore.Datastore, now time.Time) {
	fees, err := fetchFeesForPlatform(db, now)
	if err != nil {
		log.Warn(err)
	}
	trs, err := associateFeesToTransfers(db, fees, "default_for_currency", string(fee.Platform))
	if err != nil {
		panic(err)
	}
	st := makeStripeApi(db)
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

func retryIncompleteTransfers(db *datastore.Datastore) {
	errs := make([]retryError, 0, 0)
	keys, err := db.Query(transfer.Transfer{}.Kind()).
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
			err := db.Get(key, &tr)
			if err != nil {
				errs = append(errs, retryError{key, err})
			} else {
				trs = append(trs, tr)
			}
		}
		if len(errs) > 0 {
			log.Warn("failures while fetching incomplete transfers: %v", errs)
		}
		st := makeStripeApi(db)
		for _, tr := range trs {
			sendTransferToStripe(st, &tr)
		}
	}
}

func makeStripeApi(db *datastore.Datastore) *stripe.Client {
	org := organization.New(db)
	return stripe.New(db.Context, org.Stripe.AccessToken)
}

func fetchAllAffiliates(db *datastore.Datastore) []affiliate.Affiliate {
	affiliates := make([]affiliate.Affiliate, 0, 0)
	_, err := db.Query(affiliate.Affiliate{}.Kind()).GetAll(&affiliates)
	if err != nil {
		log.Error("failed to fetch affiliates: %v", err)
		panic(err)
	}
	return affiliates
}

func Run(c *gin.Context) {
	panic("XXXih: work in progress")
	db := datastore.New(c)
	retryIncompleteTransfers(db)
	now := time.Now()
	affiliates := fetchAllAffiliates(db)
	for _, aff := range affiliates {
		processAffiliateFees(db, aff, now)
	}
}
