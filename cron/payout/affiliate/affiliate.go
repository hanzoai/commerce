package affiliate

import (
	"time"

	"appengine"

	"crowdstart.com/config"
	"crowdstart.com/cron/payout"
	"crowdstart.com/datastore"
	"crowdstart.com/models/affiliate"
	"crowdstart.com/models/fee"
	"crowdstart.com/models/organization"
	"crowdstart.com/util/delay"
	"crowdstart.com/util/log"
)

// Create a copy payout.TransferFee delay.Func configured to use unique queue
var transferFee = payout.TransferFee.Queue("transfer-affiliate-fee")

// Create transfers for all un-transferred fees for associated organization
var transferFees = delay.Func("transfer-affiliate-fees", func(ctx appengine.Context, namespace, affKey string, cutoff time.Time) {
	db := datastore.New(ctx)

	// Switch namespace
	nsctx, _ := appengine.Namespace(ctx, namespace)

	// Decode affiliate key
	key, _ := datastore.DecodeKey(nsctx, affKey)

	log.Debug("Transferring affiliate fees collected in '%s'", namespace, ctx)

	// Iterate over fees that have not been transfered
	db = datastore.New(nsctx)
	q := fee.Query(db).Ancestor(key).Filter("TransferId=", "").Filter("Status=", fee.Payable).Filter("CreatedAt<", cutoff).KeysOnly()
	t := q.Run()

	for {
		key, err := t.Next(nil)

		// Done iterating
		if err == datastore.Done {
			break
		}

		// Skip field mismatch errors
		if err = datastore.IgnoreFieldMismatch(err); err != nil {
			log.Error("Failed to fetch next entity: %v", err, ctx)
			break
		}

		// Create transfer for associated fee
		transferFee.Call(ctx, config.Stripe.SecretKey, namespace, key.Encode())
	}
})

// Payout fees for all transfers
func Payout(ctx appengine.Context) error {
	db := datastore.New(ctx)

	log.Debug("Fetching all organizations", ctx)
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(db).GetAll(&orgs); err != nil {
		log.Error("Failed to fetch organizations: %v", err, ctx)
		return err
	}

	// Transfer fees for each organization
	for _, org := range orgs {
		// Switch namespace
		nsctx, _ := appengine.Namespace(ctx, org.Name)

		log.Debug("Fetching all affiliates for '%s'", org.Name, ctx)
		affs := make([]*affiliate.Affiliate, 0)
		db = datastore.New(nsctx)

		// Find all affiliates which have connected to Stripe
		if _, err := affiliate.Query(db).Filter("Stripe.AcessToken >", "").GetAll(&affs); err != nil {
			log.Error("Failed to fetch affiliates for '%s': %v", org.Name, err, ctx)
			return err
		}

		for _, aff := range affs {
			log.Debug("Processing affiliate fees for affiliate '%s', organization: '%s'", aff.Key().Encode(), org.Name, ctx)
			transferFees.Call(ctx, org.Name, aff.Key().Encode(), aff.Schedule.Cutoff(time.Now()))
		}
	}

	return nil
}
