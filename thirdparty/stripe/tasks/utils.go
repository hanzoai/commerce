package tasks

import (
	"appengine"
	"appengine/memcache"

	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
)

// Get namespaced appengine context for given namespace
func getNamespacedContext(ctx appengine.Context, ns string) appengine.Context {
	log.Debug("Setting namespace of context to %s", ns, ctx)
	ctx, err := appengine.Namespace(ctx, ns)
	if err != nil {
		log.Panic("Unable to get namespace %s: %v", ns, err, ctx)
	}
	return ctx
}

// Grab organization out of memcache
func getOrganization(ctx appengine.Context) *organization.Organization {
	org := &organization.Organization{}
	item, err := memcache.Get(ctx, "organization")
	if err != nil {
		log.Error("Failed to get organization from memcache: %v", err, ctx)
		return org
	}

	// Decode organization
	err = json.DecodeBytes(item.Value, org)
	if err != nil {
		log.Error("Failed to decode organization: %v", err, ctx)
	}
	return org
}

// Get ancestor for ancestor query for a payment associated with a stripe charge
func getPaymentFromCharge(ctx appengine.Context, ch *stripe.Charge) (*payment.Payment, bool, error) {
	db := datastore.New(ctx)
	pay := payment.New(db)

	id, ok := ch.Meta["payment"]

	// Try to get by payment id
	if ok {
		log.Debug("Try to get payment by payment id: %v", id, ctx)
		if err := pay.Get(id); err == nil {
			return pay, true, nil
		}
	}

	// Try to lookup payment using charge id
	log.Debug("Lookup payment by charge id: %v", ch.ID, ctx)
	ok, err := pay.Query().Filter("Account.ChargeId=", ch.ID).First()
	return pay, ok, err
}

// Update charge in case order/pay id is missing in metadata
func updateChargeFromPayment(ctx appengine.Context, token string, pay *payment.Payment, ch *stripe.Charge) {
	if ch != nil {
		// Check if we need to sync back changes to charge
		payId, _ := ch.Meta["payment"]
		ordId, _ := ch.Meta["order"]
		usrId, _ := ch.Meta["user"]

		// Don't sync if metadata is already correct
		if pay.Id() == payId && pay.OrderId == ordId && pay.Buyer.UserId == usrId {
			return
		}
	}

	// Get a stripe client
	client := stripe.New(ctx, token)

	// Update charge with new metadata
	if _, err := client.UpdateCharge(pay); err != nil {
		log.Error("Unable to update charge for payment '%s': %v", pay.Id(), err, ctx)
	}
}
