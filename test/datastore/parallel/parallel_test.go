package test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"github.com/zeekay/aetest"
)

var (
	ctx aetest.Context
	db  *datastore.Datastore
)

type TestCounter struct {
	Count int
}

func TestParallel(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "datastore.parallel test suite")
}

// Setup appengine context before tests
var _ = BeforeSuite(func() {
	var err error
	ctx, err = aetest.NewContext(&aetest.Options{StronglyConsistentDatastore: true})
	db := datastore.New(ctx)

	for i := 0; i < 100; i++ {
		db.Put("test-counter", &TestCounter{})
	}

	Expect(err).NotTo(HaveOccurred())
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	err := ctx.Close()
	Expect(err).NotTo(HaveOccurred())
})

// Define a new worker with parallel.Task
var TestWorker = parallel.Task("test-worker", func(db *datastore.Datastore, k datastore.Key, model TestCounter) {
	model.Count = model.Count + 1
	db.PutKey("test-counter", k, model)
})

var _ = Describe("datastore.parallel", func() {
	Context("parallel.Run", func() {
		It("should put dispatch 10 workers to process 100 entities", func() {
			parallel.Run(ctx, "parallel-test", 10, TestWorker)

			time.Sleep(10 * time.Second)

			var tcs []TestCounter
			db.Query("test-counter").GetAll(ctx, &tcs)

			Expect(len(tcs)).To(Equal(100))

			for _, tc := range tcs {
				Expect(tc.Count).To(Equal(1))
			}
		})
	})
})
