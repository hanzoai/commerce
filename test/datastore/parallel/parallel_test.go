package test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	// "github.com/zeekay/aetest"
	"github.com/mzimmerman/appenginetesting"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/test/datastore/parallel/worker"
)

var (
	ctx *appenginetesting.Context
	db  *datastore.Datastore
)

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

	db := datastore.New(ctx)

	for i := 0; i < 100; i++ {
		_, err = db.Put("test-model", &worker.Model{})
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
			parallel.Run(ctx, "parallel-test", 10, worker.Task)

			time.Sleep(1000000 * time.Second)

			var models []worker.Model
			db.Query("test-model").GetAll(ctx, &models)

			Expect(len(models)).To(Equal(100))

			for _, model := range models {
				Expect(model.Count).To(Equal(1))
			}
		})
	})
})
