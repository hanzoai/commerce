package test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"crowdstart.io/datastore"
	"github.com/zeekay/aetest"
)

func TestModels(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Models test suite")

}

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

var _ = Describe("User.GetByEmail", func() {
	Context("Should populate from datastore with valid email", func() {
	})

	Context("Should return UserNotFound when invalid email is used", func() {
	})
})
