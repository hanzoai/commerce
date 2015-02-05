package test

import (
	"path/filepath"
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
	RunSpecs(t, "datastore/parallel test suite")
	defer GinkgoRecover()

	ctx, err := appenginetesting.NewContext(&appenginetesting.Options{
		AppId:   "crowdstart-io",
		Debug:   appenginetesting.LogChild,
		Testing: t,
		Modules: []appenginetesting.ModuleConfig{
			{
				Name: "default",
				Path: filepath.Join("../../../config/development/app.yaml"),
			},
		},
	})
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}
	defer ctx.Close()

	db := datastore.New(ctx)

	// Prepoulate database with 100 entities
	for i := 0; i < 100; i++ {
		_, err = db.Put("test-model", &worker.Model{})
	}
	Expect(err).NotTo(HaveOccurred())

	// Run task in parallel
	parallel.Run(ctx, "test-model", 10, worker.Task)

	// Wait foreverrrr
	time.Sleep(1 * time.Second)

	// Check if our entities have been updated
	var models []worker.Model
	db.Query("test-model").GetAll(ctx, &models)

	Expect(len(models)).To(Equal(100))

	for _, model := range models {
		Expect(model.Count).To(Equal(1))
	}
}
