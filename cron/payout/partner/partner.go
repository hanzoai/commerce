package partner

import (
	"appengine"

	"crowdstart.com/config"
	"crowdstart.com/cron/payout"
	"crowdstart.com/datastore"
	"crowdstart.com/models/fee"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/partner"
	"crowdstart.com/util/delay"
	"crowdstart.com/util/log"
)

// Create transfer task with associated unique queue
var transferFee = delay.FuncUniq("transfer-partner-fee", payout.TransferFee)

// Create transfers for all un-transferred fees for associated partner
var transferFees = delay.Func("transfer-partner-fees", func(ctx appengine.Context, namespace, partnerId string) {
	db := datastore.New(ctx)

	// Fetch partner
	par := partner.New(db)
	if err := par.GetById(partnerId); err != nil {
		log.Error("Failed to fetch partner '%s': %v", partnerId, err, ctx)
		return
	}

	log.Debug("Transferring partner fees collected in '%s'", namespace, ctx)

	// Switch namespace
	nsctx, _ := appengine.Namespace(ctx, namespace)

	// Iterate over fees that have not been transfered
	db = datastore.New(nsctx)
	q := fee.Query(db).Ancestor(par.Key()).Filter("TransferId=", "").KeysOnly()
	t := q.Run()

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
		transferFee.Call(ctx, config.Stripe.SecretKey, namespace, key.Encode())
	}
})

// Payout partners
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
		if len(org.Partners) == 0 {
			continue
		}

		log.Debug("Processing partner fees for organization: %s", org.Name, ctx)
		for _, partner := range org.Partners {
			log.Debug("Processing partner fees for organization: '%s'", org.Name, ctx)
			transferFees.Call(ctx, org.Name, partner.Id)
		}
	}

	return nil
}
