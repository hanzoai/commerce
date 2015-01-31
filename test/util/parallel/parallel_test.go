package test

import (
	"encoding/gob"
	"testing"
	"time"

	"appengine"
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

type TestRunner struct {
	Count     int
	Something string
}

func (t TestRunner) NewObject() interface{} {
	return TestCounter{Count: t.Count}
}

func (t TestRunner) Execute(c appengine.Context, key *datastore.Key, object interface{}) error {
	t.Count++
	tc := TestCounter{Count: t.Count}
	datastore.Put(ctx, key, tc)

	log.Debug("Setting Counter to %v", t.Count, ctx)

	return nil
}

func init() {
	gob.Register(TestRunner{})
}

func TestParallel(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Parallel test suite")
}

// Setup appengine context before tests
var _ = BeforeSuite(func() {
	var err error
	ctx, err = aetest.NewContext(&aetest.Options{StronglyConsistentDatastore: true})

	for i := 0; i < 10; i++ {
		k := datastore.NewIncompleteKey(ctx, "parallel-test", nil)
		datastore.Put(ctx, k, &TestCounter{})
	}

	Expect(err).NotTo(HaveOccurred())
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	tcs := make([]TestCounter, 0)
	ks, _ := datastore.NewQuery("parallel-test").GetAll(ctx, &tcs)
	datastore.DeleteMulti(ctx, ks)

	err := ctx.Close()
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("Launch Parallel Tasks", func() {
	Context("Datastore Job", func() {
		It("should put dispatch 10 jobs (count: 1)", func() {
			parallel.DatastoreJob(ctx, "parallel-test", 1, &TestRunner{})

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
