package datastore_integration_test

import (
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/zeekay/appenginetesting"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/test/datastore/integration/tasks"
	"crowdstart.io/util/log"
)

func Test(t *testing.T) {
	log.SetVerbose(testing.Verbose())
	RegisterFailHandler(Fail)
	RunSpecs(t, "datastore/parallel")
}

var (
	ctx *appenginetesting.Context
	db  *datastore.Datastore
)

var _ = BeforeSuite(func() {
	var err error

	//Spin up an appengine dev server with the default module
	ctx, err = appenginetesting.NewContext(&appenginetesting.Options{
		AppId:      "crowdstart-io",
		Debug:      appenginetesting.LogWarning,
		Testing:    GinkgoT(),
		TaskQueues: []string{"default"},
		Modules: []appenginetesting.ModuleConfig{
			{
				Name: "default",
				Path: filepath.Join("../../../config/test/app.yaml"),
			},
		},
	})

	Expect(err).NotTo(HaveOccurred())

	// Wait for devappserver to spin up.
	time.Sleep(5 * time.Second)

	db = datastore.New(ctx)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

func checkCountValue(models []tasks.Model, v int) {
	for _, model := range models {
		Expect(model.Count).To(Equal(v))
	}
}

var _ = Describe("parallel", func() {
	It("Should run tasks in parallel", func() {
		// Prepoulate database with 10 entities
		for i := 0; i < 10; i++ {
			model := &tasks.Model{}
			_, err := db.Put("plus-1", model)
			Expect(err).NotTo(HaveOccurred())
		}

		// Run task in parallel
		parallel.Run(ctx, "plus-1", 2, tasks.TaskPlus1)

		time.Sleep(15 * time.Second)

		// Check if our entities have been updated
		var models []tasks.Model
		_, err := db.Query("plus-1").GetAll(db.Context, &models)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(models)).To(Equal(10))
		checkCountValue(models, 1)
	})
})

var _ = Describe("parallel-optional-args", func() {
	It("Should run tasks with optional arguments correctly.", func() {
		// Prepoulate database with 10 entities
		for i := 0; i < 10; i++ {
			model := &tasks.Model{}
			_, err := db.Put("set-val", model)
			Expect(err).NotTo(HaveOccurred())
		}

		// Run task in parallel
		parallel.Run(ctx, "set-val", 2, tasks.TaskSetVal, 100)

		time.Sleep(15 * time.Second)

		// Check if our entities have been updated
		var models2 []tasks.Model
		_, err := db.Query("set-val").GetAll(db.Context, &models2)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(models2)).To(Equal(10))
		checkCountValue(models2, 100)
	})
})
