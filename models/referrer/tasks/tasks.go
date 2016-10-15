package tasks

import (
	"fmt"
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

func ProcessReferralsInternal(ctx appengine.Context, referrerId string, interval TimeInterval) {
	db := datastore.New(ctx)
	// nsctx, _ := appengine.Namespace(ctx, namespace)
	oldCount, err := referral.Query(db).Filter("CreatedAt<", interval.Start).KeysOnly().Count()
	if err != nil {
		log.Error("error while fetching old count: %v", err)
		panic(err)
	}
	q := referral.Query(db).Filter("Referrer.Id=", referrerId)
	q = q.Filter("CreatedAt>=", interval.Start)
	q = q.Filter("CreatedAt<", interval.End)
	q = q.Order("CreatedAt").KeysOnly()
	t := q.Run()
	intervalCount, err := q.Count()
	if err != nil {
		log.Error("error while fetching old count: %v", err)
		panic(err)
	}
	totalCount := oldCount + intervalCount
	log.Error("hoh: %v", totalCount)
	r := referrer.New(db)
	err = r.Get(referrerId)
	if err != nil {
		log.Error("error while fetching referrer id '%s': %v", referrerId, err, ctx)
		panic(err)
	}
	currentCount := oldCount
	for {
		rflKey, err := t.Next(nil)

		if err == datastore.Done {
			break
		}

		if err = datastore.IgnoreFieldMismatch(err); err != nil {
			log.Error("referrer: failed to fetch next entity: %v", err, ctx)
			break
		}

		counter.IncrReferrerTotal(ctx, referrerId)
		nextCount := currentCount + 1
		errors := r.Program.ApplyActions(r, currentCount, nextCount)
		if len(errors) > 0 {
			log.Error("errors while applying program actions for referral '%s': %v", rflKey, errors, ctx)
			return
		}
		currentCount = nextCount
	}
}

// Process a block of referrals.
// This sorts and processes referrals in approximate temporal order.
// Referral records are assigned a timestamp using each request handler's
// local time, so this function should be scheduled to run with a delay
// greater than the total expected clock drift between the "earliest"
// request-handling node and the "latest" request-handling node.
var saveReferral_ = delay.Func("", func(ctx appengine.Context, namespace string, interval TimeInterval) {
	ProcessReferralsInternal(ctx, namespace, interval)
})

type TimeInterval struct {
	Start time.Time // inclusive
	End   time.Time // exclusive
}

func IntervalFromInstant(t time.Time, width time.Duration) TimeInterval {
	start := t.Truncate(intervalWidth)
	end := start.Add(intervalWidth)
	return TimeInterval{start, end}
}

func NameFromInterval(d time.Duration, i TimeInterval) string {
	layout := time.RFC3339
	start := i.Start.Format(layout)
	end := i.End.Format(layout)
	// taskqueue supports delays specified in terms of microseconds:
	// https://github.com/golang/appengine/blob/75a29a66d4850a15c19eb6d70a31f5c453572be0/internal/taskqueue/taskqueue_service.pb.go#L456
	// https://github.com/golang/appengine/blob/75a29a66d4850a15c19eb6d70a31f5c453572be0/taskqueue/taskqueue.go#L176
	return fmt.Sprintf("processReferrals-%d-from-%s-to-%s", int64(d.Nanoseconds() / 1000), start, end)
}

const processingLatency = 5 * time.Minute
const intervalWidth = 10 * time.Minute

func ProcessReferrals(ctx appengine.Context, t time.Time) {
	interval := IntervalFromInstant(t, intervalWidth)
	name := NameFromInterval(processingLatency, interval)
	saveReferral_.Once(ctx, name, processingLatency, interval)
}

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
		if err = counter.IncrReferrerFees(ctx, r.Id(), rfl); err != nil {
			log.Warn("IncrReferrerFees: counter error %s", err, ctx)
		}
		if r.AffiliateId != "" {
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
