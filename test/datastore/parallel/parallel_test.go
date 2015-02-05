package test

import (
	"testing"
	"time"

	"appengine/datastore"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"crowdstart.io/util/log"
	"crowdstart.io/util/parallel"
	"github.com/zeekay/aetest"
)

var (
	ctx aetest.Context
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

	for i := 0; i < 100; i++ {
		k := datastore.NewIncompleteKey(ctx, "parallel-test", nil)
		datastore.Put(ctx, k, &TestCounter{})
	}

	Expect(err).NotTo(HaveOccurred())
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	err := ctx.Close()
	Expect(err).NotTo(HaveOccurred())
})

var TestWorker = parallel.Task(func(db *datastore.Datastore, k datastore.Key, model TestCounter) {
	model.Count = model.Count + 1
	db.Put(k, model)
})

var _ = Describe("Launch Parallel Tasks", func() {
	Context("Datastore Job", func() {
		It("should put dispatch 10 jobs (count: 10)", func() {
			parallel.DatastoreJob(ctx, "parallel-test", 10, &TestRunner{})

			time.Sleep(1 * time.Second)

			tcs := make([]TestCounter, 0)
			datastore.NewQuery("parallel-test").GetAll(ctx, &tcs)

			log.Debug("Length %v", len(tcs), ctx)
			Expect(len(tcs)).To(Equal(10))
			for _, tc := range tcs {
				Expect(tc.Count).To(Equal(0 /*1*/))
			}
		})
	})
})
