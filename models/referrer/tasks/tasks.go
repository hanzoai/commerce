package tasks

import (
	"appengine"

	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/referrer/process_program"
	"crowdstart.com/util/counter"
	"crowdstart.com/util/delay"
	"crowdstart.com/util/log"
)

type EventType string

const (
	Checkout EventType = "checkout"
	Signup             = "signup"
)

// Process a block of referrals.
// This sorts and processes referrals in approximate temporal order.
// Referral records are assigned a timestamp using each request handler's
// local time, so this function should be scheduled to run with a delay
// greater than the total expected clock drift between the "earliest"
// request-handling node and the "latest" request-handling node.
func DoBatchProcessReferrals(ctx appengine.Context, namespace string, interval referrer.TimeInterval, referrerId string, orgId string) {
	db := datastore.New(ctx)
	// nsctx, _ := appengine.Namespace(ctx, namespace)

	org := organization.New(db)
	err := org.GetById(orgId)

	if err != nil {
		log.Error("error while fetching organization; orgId = '%s': %v", orgId, err, ctx)
		panic(err)
	}

	r := referrer.New(db)
	err = r.GetById(referrerId)
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
		errors := process_program.ApplyActions(org, &r.Program, r, currentCount, nextCount)
		if len(errors) > 0 {
			log.Error("errors while applying program actions for referral '%s': %v", rflKey, errors, ctx)
			return
		}
		currentCount = nextCount
	}
}

var BatchProcessReferrals = delay.Func("batch-process-referrals", func(ctx appengine.Context, namespace string, interval referrer.TimeInterval, referrerId string, orgId string) {
	DoBatchProcessReferrals(ctx, namespace, interval, referrerId, orgId)
})

