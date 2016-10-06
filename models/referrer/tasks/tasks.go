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

type LostRace struct {
}

func (l LostRace) Error() string {
	return "lost race"
}

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

	switch evtType {
	case Checkout:
		// Update statistics
		if r.AffiliateId != "" {
			if err = counter.IncrReferrerFees(ctx, r.Id(), rfl); err != nil {
				log.Warn("IncrReferrerFees: counter error %s", err, ctx)
			}

			if err = counter.IncrAffiliateFees(ctx, r.AffiliateId, rfl); err != nil {
				log.Warn("IncrAffiliateFees: counter error %s", err, ctx)
			}
		}
	}

	// Fulfill rewards according to the referrer's reward program.
	// The sequence of steps is:
	// 1. increment the referral count;
	// 2. fetch the referral count;
	// 3. perform an atomic compare-exchange (implemented via a transaction)
	//    to update the stored reward count and fetch the old stored reward
	//    count, such that:
	//    a. a failure on reward fulfillment will not double-award a reward, and
	//    b. a concurrent referrer processor will not double-award a reward;
	// 4. apply the eligible rewards.
	//
	// The current implementation has the disadvantage that referral
	// updates are throughput-limited by datastore: the RewardedReferrals
	// field of the Referrer record is updated on every single referral.
	// Additionally, it is still possible for rewards to fail to deliver
	// after the stored reward count has been updated, which means that
	// these failures must be manually remediated.
	//
	// A possible solution would entail giving reward programs unique IDs,
	// making them immutable, and then storing all fulfilled action IDs
	// (e.g., a (program-id, action-array-index) tuple) inside a
	// set. If this is done, reward fulfillment easily becomes restartable
	// and idempotent: reward IDs would first be sharded across multiple
	// entity groups (to prevent entity group write contention), and
	// then individual reward fulfillment could be done in a cross-entity
	// transaction.
	if err = counter.IncrReferrerTotal(ctx, r.Id()); err != nil {
		log.Warn("IncrReferrerTotal: Counter error %s", err, ctx)
	}
	if len(r.Program.Triggers) > 0 {
		referrals, err := counter.GetReferrerTotal(ctx, r.Id())
		if err != nil {
			log.Warn("GetReferrerTotal: did not apply program actions; counter error %s", err, ctx)
			return
		}
		var rewardedReferrals int
		// Note: mixin.RunInTransaction can only be used because only
		// one "model" is being transactionally modified. This API is
		// flawed in the general case, and should either take a list of
		// all "models" to be used in the transaction, or must
		// involve explicit context passing. Context passing has the
		// additional advantage of being thread-safe.
		// (Context passing can be automated away with a reader monad,
		// which alleviates the syntactic burden of context passing and
		// makes it impossible to mismatch contexts inside a
		// transaction. Sadly, we're using go.)
		err = r.RunInTransaction(func () error {
			err := r.GetById(r.Id())
			if err != nil {
				return err
			}
			if (r.RewardedReferrals >= referrals) {
				return LostRace{}
			}
			rewardedReferrals = r.RewardedReferrals
			r.RewardedReferrals = referrals
			r.Update()
			return nil
		})
		switch err.(type) {
		case LostRace:
			// another thread completed steps 1-3 while the current
			// thread was executing after-step-2-but-before-step-3;
			// thus, the other thread will deliver the rewards that
			// would have otherwise been handled by this thread
			return
		case nil:
		default:
			log.Warn("r.Update(): failed while updating processed referral count: %s", err, ctx)
			return
		}
		errors := r.Program.ApplyActions(r, rewardedReferrals, referrals)
		if len(errors) > 0 {
			log.Warn("r.Program.PerformActions(): failure while performing actions: %s", errors, ctx)
		}
	}
})

func SaveReferral(ctx appengine.Context, referrerId string, evtType EventType) {
	saveReferral.Call(ctx, referrerId, evtType)
}
