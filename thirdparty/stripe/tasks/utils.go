package tasks

import (
	"errors"
	"fmt"

	"appengine"
	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/hashid"
	"crowdstart.com/util/log"
)

func getNamespace(ctx appengine.Context, ns string) appengine.Context {
	log.Debug("Setting namespace of context to %s", ns, ctx)
	ctx, err := appengine.Namespace(ctx, ns)
	if err != nil {
		log.Panic("Unable to get namespace %s: %v", ns, err, ctx)
	}
	return ctx
}

func getAncestor(ctx appengine.Context, ch stripe.Charge) (*aeds.Key, error) {
	// Try to user order id if possible
	if id, ok := ch.Meta["order"]; ok {
		log.Debug("Try to use order id in charge metadata", ctx)
		return hashid.DecodeKey(ctx, id)
	}

	// Try to lookup payment
	db := datastore.New(ctx)
	pay := payment.New(db)
	var err error

	id, ok := ch.Meta["payment"]

	// Try to get by payment id
	if ok {
		log.Debug("Try to get payment by payment id: %v", id, ctx)
		err = pay.Get(id)
	}

	// Lookup by charge id
	if !ok || err != nil {
		log.Debug("Lookup payment by charge id: %v", ch.ID, ctx)
		_, err = pay.Query().Filter("Account.ChargeId=", ch.ID).First()
	}

	if err != nil {
		log.Debug("Unable to lookup payment id", ctx)
		return nil, errors.New(fmt.Sprintf("Unable to lookup payment by id (%s) or charge id (%s): %v", id, ch.ID, err, ctx))
	}

	log.Debug("Try to decode order id: %v", pay.OrderId, ctx)
	return hashid.DecodeKey(ctx, pay.OrderId)
}
