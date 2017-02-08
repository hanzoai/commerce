package datastore_integration_test

import (
	"errors"
	"fmt"
	"testing"

	"golang.org/x/net/context"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/test/datastore/integration/tasks"
	"hanzo.io/test/fixtures/user"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/log"
	"hanzo.io/util/test/ae"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("datastore/integration", t)
}

var (
	c    *gin.Context
	ctx  context.Context
	inst ae.Instance
	db   *datastore.Datastore
)

var _ = BeforeSuite(func() {
	var err error
	ctx, inst, err = ae.NewContext(ae.Options{
		Modules:    []string{"default"},
		TaskQueues: []string{"default"},
		LogChild:   true,
	})
	Expect(err).NotTo(HaveOccurred())

	c = gincontext.New(ctx)
	db = datastore.New(ctx)
})

var _ = AfterSuite(func() {
	inst.Close()
})

func checkCountValue(filter string, numModels int, expected int) {
	err := Retry(20, func() error {
		slice, err := user.New(db).Query().Filter("Name=", filter).GetAll()
		if err != nil {
			log.Error("Failed to get models from datastore: %v", err)
			return err
		}

		models := slice.([]*user.User)

		Expect(len(models)).To(Equal(numModels))

		// Make sure expected count is right
		for _, model := range models {
			if model.Count != expected {
				return errors.New(fmt.Sprintf("Task did not set value on model correctly, expected: %v, found: %v, models: %#v", expected, model.Count, models))
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
				model := user.New(db)
				model.Name = "parallel-a"
				err := model.Put()
				Expect(err).NotTo(HaveOccurred())
			}

			// Run task in parallel
			tasks.TaskPlus1.Run(c, 2)

			// Check if our entities have been updated
			checkCountValue("parallel-a", 10, 1)
		})
	})

	Context("With task taking optional argument", func() {
		It("Should run tasks in parallel", func() {
			// Prepoulate database with 10 entities
			for i := 0; i < 10; i++ {
				model := user.New(db)
				model.Name = "parallel-b"
				err := model.Put()
				Expect(err).NotTo(HaveOccurred())
			}

			// Run task in parallel
			tasks.TaskSetVal.Run(c, 2, 100)

			checkCountValue("parallel-b", 10, 100)
		})
	})

	Context("With task filter", func() {
		It("Should run tasks in parallel", func() {
			// Prepoulate database with 10 entities
			for i := 0; i < 10; i++ {
				model := user.New(db)
				model.Name = "parallel-c"
				model.Count2 = i
				model.Count = i
				err := model.Put()
				Expect(err).NotTo(HaveOccurred())
			}

			// Run task in parallel
			tasks.TaskSetFilter.Run(c, 2, 100)

			err := Retry(20, func() error {
				slice, err := user.New(db).Query().Filter("Name=", "parallel-c").GetAll()
				if err != nil {
					log.Error("Failed to get models from datastore: %v", err)
					return err
				}

				models := slice.([]*user.User)
				Expect(len(models)).To(Equal(10))
				for _, model := range models {
					log.Warn("Model.Count %v", model.Count)
					log.Warn("Model.Count2 %v", model.Count2)
				}

				// Make sure expected count is right
				for i, model := range models {
					if model.Count < 5 {
						if model.Count2 != 100 {
							return errors.New(fmt.Sprintf("Task did not set value on model correctly, expected: %v, found: %v, models: %#v", 100, model.Count, models))
						}
					} else {
						if model.Count2 != i {
							return errors.New(fmt.Sprintf("Task did not set value on model correctly, expected: %v, found: %v, models: %#v", 100, model.Count, models))
						}
					}
				}

				return nil
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
