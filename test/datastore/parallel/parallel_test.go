package test

import (
	"path/filepath"
	"testing"
	"time"

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

	// Wait for devappserver to spin up.
	time.Sleep(30 * time.Second)

	db := datastore.New(ctx)

	// Prepoulate database with 100 entities
	for i := 0; i < 10; i++ {
		if _, err = db.Put("test-model", &worker.Model{}); err != nil {
			t.FailNow()
		}
	}

	// Run task in parallel
	parallel.Run(ctx, "test-model", 2, worker.Task)

	// Wait foreverrrr
	time.Sleep(10 * time.Second)

	// Check if our entities have been updated
	var models []worker.Model
	db.Query("test-model").GetAll(ctx, &models)

	if len(models) != 10 {
		t.FailNow()
	}

	for _, model := range models {
		if model.Count != 1 {
			t.FailNow()
		}
	}
}
