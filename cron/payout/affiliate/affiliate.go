package affiliate

import (
	"appengine"

	"crowdstart.com/cron/payout"
	"crowdstart.com/datastore"
	"crowdstart.com/models/fee"
	"crowdstart.com/models/organization"
	"crowdstart.com/util/delay"
	"crowdstart.com/util/log"
)

// Create transfer task with associated unique queue
var transferFee = delay.FuncUniq("transfer-affiliate-fee", payout.TransferFee)

// Create transfers for all un-transferred fees for associated organization
var transferFees = delay.Func("transfer-affiliate-fees", func(ctx appengine.Context, id string) {
	db := datastore.New(ctx)

	// Fetch organization
	org := organization.New(db)
	if err := org.GetById(id); err != nil {
		log.Panic("Failed to fetch organization by id: '%s'", err)
	}

	log.Debug("Fetching affiliate fees for organization: %s", org.Name, ctx)

	nsctx := org.Namespaced(ctx)
	db = datastore.New(nsctx)
	q := fee.Query(db).Ancestor(org.Key()).Filter("TransferId=", "").KeysOnly()
	t := q.Run()

	// Loop over entities passing them into workerFunc one at a time
	for {
		key, err := t.Next(nil)

		// Done iterating
		if err == datastore.Done {
			break
		}

		// Skip field mismatch errors
		if err := db.SkipFieldMismatch(err); err != nil {
			log.Error("Failed to fetch next entity: %v", err, ctx)
			break
		}

		// Create transfer for associated fee
		transferFee.Call(ctx, org.Stripe.AccessToken, org.Name, key.Encode())
	}
})

// Payout fees for all transfers
func Payout(ctx appengine.Context) error {
	db := datastore.New(ctx)

	log.Debug("Fetching all organizations", ctx)
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(db).GetAll(&orgs); err != nil {
		log.Error("Failed to fetch organizations", ctx)
		return err
	}

	// Transfer fees for each organization
	for _, org := range orgs {
		log.Debug("Processing affiliate fees for affiliate: %s", org.Name, ctx)
		transferFees.Call(ctx, org.Id())
	}

	return nil
}
