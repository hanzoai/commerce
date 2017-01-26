package review

import (
	"crowdstart.com/datastore"
	"crowdstart.com/util/fake"
)

func Fake(db *datastore.Datastore, userId, productId string) *Review {
	r := New(db)

	r.UserId = userId
	r.ProductId = productId

	r.Name = fake.FullName()
	r.Comment = fake.Comment()
	r.Rating = fake.Rating(5)

	return r
}
