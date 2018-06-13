package subscription

import (
	"time"
	"math/rand"

	"hanzo.io/datastore"
	"hanzo.io/models/plan"
	"hanzo.io/util/fake"
)

func Fake(db *datastore.Datastore) *Subscription {
	sub := New(db)
	sub.PlanId = fake.Id()
	sub.UserId = fake.Id()
	sub.FeePercent = fake.Percent
	sub.PeriodStart = time.Now()
	sub.PeriodEnd = time.Now().AddDate(0,0,30)
	sub.Canceled = false
	sub.Quantity = rand.Intn(10)
	sub.Status = Active
	sub.Plan = *plan.Fake(db)
	return sub
}
