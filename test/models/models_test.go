package test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"time"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"github.com/zeekay/aetest"
)

var (
	ctx aetest.Context
	db  *datastore.Datastore
)

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	var err error
	ctx, err = aetest.NewContext(&aetest.Options{StronglyConsistentDatastore: true})
	Expect(err).NotTo(HaveOccurred())
	db = datastore.New(ctx)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	err := ctx.Close()
	Expect(err).NotTo(HaveOccurred())
})

func TestDatastore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "models test suite")
}

type User struct {
	mixin.Model `datastore:"-"`
	Name        string
}

func (u *User) Kind() string {
	return "user"
}

func NewUser(db *datastore.Datastore) *User {
	user := new(User)
	user.Model = mixin.NewModel(db, user)
	return user
}

var _ = Describe("mixin", func() {
	It("should work", func() {
		// Usage
		user := NewUser(db)
		user.Name = "Justin"
		user.Put()
		time.Sleep(10 * time.Second)
		user2 := new(User)
		db.Get(user.Key(), user2)
		Expect(user2.Name).To(Equal(user.Name))
	})
})
