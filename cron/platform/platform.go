package platform

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/fee"
	"crowdstart.com/models/fee/tasks"
	"crowdstart.com/models/organization"
	"crowdstart.com/util/log"

	"appengine"
	"appengine/delay"
)

var payoutPlatformByOrg = delay.Func("payout-platform-by-org", func(ctx appengine.Context, id string) {
	db := datastore.New(ctx)
	org := organization.New(db)
	if err := org.GetById(id); err != nil {
		log.Panic("Failed to fetch organization by id: '%s'", err)
	}

	nsctx := org.Namespaced(ctx)
	db = datastore.New(nsctx)

	log.Debug("Processing platform fees for organization: %s", org.Name, ctx)
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

		tasks.PayoutPlatform.Call(ctx, org.Stripe.AccessToken, org.Name, key.Encode())
	}
})

func Payout(ctx appengine.Context) error {
	db := datastore.New(ctx)

	log.Debug("Fetching all organizations", ctx)
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(db).GetAll(&orgs); err != nil {
		log.Error("Failed to fetch organizations", ctx)
		return err
	}

	for _, org := range orgs {
		payoutPlatformByOrg.Call(ctx, org.Id())
	}

	return nil
}
