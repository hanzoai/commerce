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
)

type EventType string

const (
	Checkout EventType = "checkout"
	Signup             = "signup"
)

func ProcessReferralsInternal(ctx appengine.Context, referrerId string, interval TimeInterval) {
	db := datastore.New(ctx)
	// nsctx, _ := appengine.Namespace(ctx, namespace)

	r := referrer.New(db)
	err := r.GetById(referrerId)
	if err != nil {
		log.Error("error while fetching referrer; referrerId = '%s': %v", referrerId, err, ctx)
		panic(err)
	}

	oldCount, err := counter.GetReferrerTotal(ctx, r.Id())
	if err != nil {
		log.Error("error while fetching old count: %v", err)
		panic(err)
	}

	q := referral.Query(db).Filter("Referrer.Id=", referrerId)
	q = q.Filter("CreatedAt>=", interval.Start)
	q = q.Filter("CreatedAt<", interval.End)
	q = q.Order("CreatedAt").KeysOnly()
	t := q.Run()

	currentCount := oldCount
	for {
		rflKey, err := t.Next(nil)

		if err == datastore.Done {
			break
		}

		if err = datastore.IgnoreFieldMismatch(err); err != nil {
			log.Error("failed to fetch next entity: %v", err, ctx)
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
