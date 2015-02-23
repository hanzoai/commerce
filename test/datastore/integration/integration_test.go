package datastore_integration_test

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/test/datastore/integration/tasks"
	"crowdstart.io/util/gincontext"
	"crowdstart.io/util/test/ae"
	"crowdstart.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	ginkgo.Setup("datastore/integration", t)
}

var (
	c   *gin.Context
	ctx ae.Context
)

var _ = BeforeSuite(func() {
	ctx = ae.NewContext(ae.Options{
		Modules:    []string{"default"},
		TaskQueues: []string{"default"},
	})

	c = gincontext.New(ctx)

	// Wait for devappserver to spin up.
	time.Sleep(3 * time.Second)
})

var _ = AfterSuite(func() {
	ctx.Close()
})

func checkCountValue(models []tasks.Model, v int) {
	for _, model := range models {
		Expect(model.Count).To(Equal(v))
	}
}

var _ = Describe("datastore/parallel", func() {
	Context("With task", func() {
		It("Should run tasks in parallel", func() {
			db := datastore.New(ctx)

			// Prepoulate database with 10 entities
			for i := 0; i < 10; i++ {
				model := &tasks.Model{}
				_, err := db.Put("plus-1", model)
				Expect(err).NotTo(HaveOccurred())
			}

			// Run task in parallel
			parallel.Run(c, "plus-1", 2, tasks.TaskPlus1)

			time.Sleep(12 * time.Second)

			// Check if our entities have been updated
			var models []tasks.Model
			_, err := db.Query("plus-1").GetAll(db.Context, &models)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(models)).To(Equal(10))
			checkCountValue(models, 1)
		})
	})

	Context("With task taking optional argument", func() {
		It("Should run tasks in parallel", func() {
			db := datastore.New(ctx)

			// Prepoulate database with 10 entities
			for i := 0; i < 10; i++ {
				model := &tasks.Model{}
				_, err := db.Put("set-val", model)
				Expect(err).NotTo(HaveOccurred())
			}

			// Run task in parallel
			parallel.Run(c, "set-val", 2, tasks.TaskSetVal, 100)

			time.Sleep(12 * time.Second)

			// Check if our entities have been updated
			var models2 []tasks.Model
			_, err := db.Query("set-val").GetAll(db.Context, &models2)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(models2)).To(Equal(10))
			checkCountValue(models2, 100)
		})
	})
})
