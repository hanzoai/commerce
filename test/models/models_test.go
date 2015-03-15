package test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/test/ae"
	"crowdstart.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	ginkgo.Setup("models", t)
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

type User struct {
	mixin.Model
	mixin.AccessTokener

	Name string
}

func (u *User) Kind() string {
	return "user"
}

func NewUser(db *datastore.Datastore) *User {
	u := new(User)
	u.Model = mixin.Model{Db: db, Entity: u}
	u.AccessTokener = mixin.AccessTokener{Model: u}
	return u
}

var _ = Describe("models/mixin", func() {
	Context("Model.Put", func() {
		It("should save entity to datastore", func() {
			// Create a new user and store using Model mixin
			user := NewUser(db)
			user.Name = "Justin"
			user.Put()

			// Manually retrieve to ensure it was saved properly
			user2 := new(User)
			db.Get(user.Key(), user2)
			Expect(user2.Name).To(Equal(user.Name))
		})
	})

	Context("Model.Get", func() {
		It("should retrieve entity from datastore", func() {
			// Manually create a new user and store in datastore
			user := new(User)
			user.Name = "Dustin"
			key, err := db.Put("user", user)
			Expect(err).NotTo(HaveOccurred())

			// Retrieve user from datastore using Model mixin
			user2 := NewUser(db)
			user2.Get(key)
			Expect(user2.Name).To(Equal(user.Name))
		})
	})

	Context("AccessTokener.GenerateAccessToken/GetWithAccessToken", func() {
		It("Should be able to create and validate AccessToken", func() {
			// Create a new user and store using Model mixin
			user := NewUser(db)
			user.Name = "Justin"
			user.IssuedAt = time.Now()
			user.SecretKey = []byte("AAA")

			// Create the token for looking up
			tokenStr, err := user.GenerateAccessToken()
			Expect(err).NotTo(HaveOccurred())

			user.Put()

			// Manually retrieve to ensure it was saved properly
			user2 := NewUser(db)
			err = mixin.GetWithAccessToken(tokenStr, &user2.AccessTokener)
			Expect(err).NotTo(HaveOccurred())
			Expect(user2.Name).To(Equal(user.Name))
		})

		It("Should be able to invalidate AccessToken by creating a new one", func() {
			// Create a new user and store using Model mixin
			user := NewUser(db)
			user.Name = "Justin"
			user.IssuedAt = time.Now()
			user.SecretKey = []byte("AAA")

			// Create the token for looking up
			invalidTokenStr, err := user.GenerateAccessToken()
			Expect(err).NotTo(HaveOccurred())

			validTokenStr, err := user.GenerateAccessToken()
			Expect(err).NotTo(HaveOccurred())

			user.Put()

			// Manually retrieve to ensure it was saved properly
			user2 := NewUser(db)
			err = mixin.GetWithAccessToken(invalidTokenStr, &user2.AccessTokener)
			Expect(err).To(HaveOccurred())

			err = mixin.GetWithAccessToken(validTokenStr, &user2.AccessTokener)
			Expect(err).NotTo(HaveOccurred())

			Expect(user2.Name).To(Equal(user.Name))
		})
	})
})
