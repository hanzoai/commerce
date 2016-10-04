package tasks

import (
	"time"

	"appengine"

	"crowdstart.com/datastore"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/referrer"
	"crowdstart.com/util/counter"
	"crowdstart.com/util/delay"
	"crowdstart.com/util/log"
	"crowdstart.com/util/timeutil"
)

type EventType string

const (
	Checkout EventType = "checkout"
	Signup             = "signup"
)

var saveReferral = delay.Func("save-referral", func(ctx appengine.Context, referrerId string, evtType EventType, data interface{}) {
	db := datastore.New(ctx)

	r := referrer.New(db)
	err := r.GetById(referrerId)
	if err != nil {
		panic(err)
	}

	rfl := referral.New(db)

	rfl.Referrer.UserId = r.UserId
	rfl.Referrer.Id = referrerId

	// switch v := referent.(type) {
	// case *order.Order:
	// 	rfl.OrderId = v.Id()
	// case *user.User:
	// 	rfl.UserId = v.Id()
	// }

	// Try to save referral
	if err = rfl.Create(); err != nil {
		log.Warn("Unable to create referral with referral id = %v", referrerId)
		panic(err)
	}

	// If this is the first referral, update referrer
	if timeutil.IsZero(r.FirstReferredAt) {
		r.FirstReferredAt = time.Now()
		r.Update()
	}

	// Apply any program actions if they are configured
	if len(r.Program.Actions) > 0 {
		if err = r.Program.ApplyActions(r); err != nil {
			log.Warn("Unable to save referral with id %v", err, referrerId)
			panic(err)
		}
	}

	switch evtType {
	case Checkout:
		// Update statistics
		if r.AffiliateId != "" {
			if err = counter.IncrReferrerFees(ctx, r.Id(), rfl); err != nil {
				log.Warn("Counter Error %s", err, ctx)
			}

			if err = counter.IncrAffiliateFees(ctx, r.AffiliateId, rfl); err != nil {
				log.Warn("Counter Error %s", err, ctx)
			}
		}
	}
})

func SaveReferral(ctx appengine.Context, referrerId string, evtType EventType) {
	saveReferral.Call(ctx, referrerId, evtType)
}
