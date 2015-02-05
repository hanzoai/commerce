package test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"

	// "github.com/zeekay/aetest"
	"github.com/mzimmerman/appenginetesting"
)

var (
	ctx *appenginetesting.Context
	db  *datastore.Datastore
)

type TestCounter struct {
	Count int
}

func TestParallel(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "datastore.parallel test suite")
	ctx, _ = appenginetesting.NewContext(&appenginetesting.Options{
		Debug:   appenginetesting.LogDebug,
		Testing: t,
	})
}

// Setup appengine context before tests
var _ = BeforeSuite(func() {
	var err error

	Expect(err).NotTo(HaveOccurred())

	// ctx, err = aetest.NewContext(&aetest.Options{StronglyConsistentDatastore: true})
	db := datastore.New(ctx)

	for i := 0; i < 100; i++ {
		db.Put("test-counter", &TestCounter{})
	}

	Expect(err).NotTo(HaveOccurred())
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("datastore.parallel", func() {
	Context("parallel.Run", func() {
		It("should put dispatch 10 workers to process 100 entities", func() {
			parallel.Run(ctx, "parallel-test", 10, TestWorker)

			time.Sleep(1000000 * time.Second)

			var tcs []TestCounter
			db.Query("test-counter").GetAll(ctx, &tcs)

			Expect(len(tcs)).To(Equal(100))

			for _, tc := range tcs {
				Expect(tc.Count).To(Equal(1))
			}
		})
	})
})
