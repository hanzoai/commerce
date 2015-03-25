package datastore_integration_test

import (
	"errors"
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
	db  *datastore.Datastore
)

var _ = BeforeSuite(func() {
	ctx = ae.NewContext(ae.Options{
		Modules:    []string{"default"},
		TaskQueues: []string{"default"},
	})

	c = gincontext.New(ctx)
	db = datastore.New(ctx)
})

var _ = AfterSuite(func() {
	ctx.Close()
})

func checkCountValue(kind string, numModels int, expected int) {
	err := Retry(10, func() error {
		var models []tasks.Model
		_, err := db.Query(kind).GetAll(db.Context, &models)
		if err != nil {
			return err
		}

		Expect(len(models)).To(Equal(numModels))

		// Make sure expected count is right
		for _, model := range models {
			if model.Count != expected {
				return errors.New("Task did not set value on model correctly.")
			}
		}

		return nil
	})
	Expect(err).NotTo(HaveOccurred())
}

var _ = Describe("datastore/parallel", func() {
	Context("With task", func() {
		It("Should run tasks in parallel", func() {
			// Prepoulate database with 10 entities
			for i := 0; i < 10; i++ {
				_, err := db.Put("plus-1", &tasks.Model{})
				Expect(err).NotTo(HaveOccurred())
			}

			// Run task in parallel
			parallel.Run(c, "plus-1", 2, tasks.TaskPlus1)

			// Check if our entities have been updated
			checkCountValue("plus-1", 10, 1)
		})
	})

	Context("With task taking optional argument", func() {
		It("Should run tasks in parallel", func() {
			// Prepoulate database with 10 entities
			for i := 0; i < 10; i++ {
				model := &tasks.Model{}
				_, err := db.Put("set-val", model)
				Expect(err).NotTo(HaveOccurred())
			}

			// Run task in parallel
			parallel.Run(c, "set-val", 2, tasks.TaskSetVal, 100)

			checkCountValue("set-val", 10, 100)
		})
	})
})
