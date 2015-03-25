package datastore_integration_test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/test/datastore/integration/tasks"
	"crowdstart.io/util/gincontext"
	"crowdstart.io/util/test/ae"

	. "crowdstart.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("datastore/integration", t)
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

			// Check if our entities have been updated
			var models []tasks.Model
			var err error
			Retry(5, func() error {
				_, err = db.Query("plus-1").GetAll(db.Context, &models)
				return err
			})
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

			var err error
			var models2 []tasks.Model
			Retry(5, func() error {
				_, err := db.Query("set-val").GetAll(db.Context, &models2)
				return err
			})

			// Check if our entities have been updated
			Expect(err).NotTo(HaveOccurred())
			Expect(len(models2)).To(Equal(10))
			checkCountValue(models2, 100)
		})
	})
})
