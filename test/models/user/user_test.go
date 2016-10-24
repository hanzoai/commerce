package test

import (
	"testing"

	"crowdstart.com/datastore"
	"crowdstart.com/models/user"
	"crowdstart.com/util/test/ae"

	. "crowdstart.com/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("models/user", t)
}

var (
	ctx ae.Context
	db  *datastore.Datastore
)

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	db = datastore.New(ctx)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("User", func() {
	It("should retrieve entity from datastore by email", func() {
		// Manually create a new user and store in datastore
		usr := user.Fake(db)
		usr.MustCreate()

		// Retrieve usr from datastore using Model mixin
		usr2 := user.New(db)
		usr2.MustGetById(usr.Email)
		Expect(usr2.Email).To(Equal(usr2.Email))
		Expect(usr2.Name()).To(Equal(usr2.Name()))
	})
})
