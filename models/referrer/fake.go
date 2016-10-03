package referrer

import (
	"math/rand"
	"time"

	"crowdstart.com/datastore"
	"crowdstart.com/util/fake"
)

func Fake(db *datastore.Datastore, userId, orderId string) *Referrer {
	r := New(db)
	r.Code = fake.Word()
	r.OrderId = orderId
	r.UserId = userId
	r.FirstReferredAt = time.Date(rand.Intn(15)+2000, time.Month(rand.Intn(12)+1), rand.Intn(25)+1, 0, 0, 0, 0, time.UTC)

	return r
}
