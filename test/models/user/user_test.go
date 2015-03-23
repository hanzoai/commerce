package test

import (
	"testing"

	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/test/ae"

	. "crowdstart.io/util/test/ginkgo"
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
	Context("Insert", func() {
		It("Should insert a new user", func() {
			// Insert User
			user := &models.User{Email: "u1@verus.io"}
			user.Insert(db)

			// Get User and Compare Email
			var _user models.User
			db.Get(user.Id, &_user)
			Expect(_user.Email).To(Equal(user.Email))
		})
	})

	Context("Upsert", func() {
		It("Should upsert a user and overwrite what is in the datastore", func() {
			// Insert Via Upsert User
			user := &models.User{Email: "u2@verus.io"}
			user.Upsert(db)

			// Get User and Compare Email
			var _user models.User
			db.Get(user.Id, &_user)
			Expect(_user.Email).To(Equal(user.Email))

			// Change Email on User and Upsert User
			user.Email = "u3@verus.io"
			user.Upsert(db)

			// Get User and Compare Changed Email
			var __user models.User
			db.Get(user.Id, &__user)
			Expect(__user.Email).To(Equal(user.Email))
		})

		It("Should upsert a user and overwrite what is in the datastore based on email if id is missing", func() {
			// Insert Via Upsert User
			user := &models.User{Email: "u2@verus.io", FirstName: "u2"}
			user.Upsert(db)

			// Get User and Compare Email and FirstName
			var _user models.User
			db.Get(user.Id, &_user)
			Expect(_user.Email).To(Equal(user.Email))
			Expect(_user.FirstName).To(Equal(user.FirstName))

			// Change Email on User and Upsert User
			user2 := models.User{Email: "u2@verus.io", FirstName: "u3"}
			user2.Upsert(db)

			// Get User and Compare Changed FirstName, Email, and Id
			var __user models.User
			db.Get(user2.Id, &__user)
			Expect(__user.Id).To(Equal(_user.Id))
			Expect(__user.Email).To(Equal(user.Email))
			Expect(__user.FirstName).To(Equal(user2.FirstName))
		})

		Context("GetByEmail", func() {
			It("Should be able to GetUserByEmail", func() {
				// Insert User
				user := &models.User{Email: "u1@verus.io"}
				user.Insert(db)

				// Get User by Email and Check Email
				_user := &models.User{}
				_ = _user.GetByEmail(db, "u1@verus.io")
				Expect(_user.Email).To(Equal(user.Email))
			})
		})
	})
})
