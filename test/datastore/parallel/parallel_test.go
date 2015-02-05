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
		t.Fatalf("Failed to create appengine context: %v", err)
	}
	defer ctx.Close()

	// Wait for devappserver to spin up.
	time.Sleep(20 * time.Second)

	db := datastore.New(ctx)

	// Prepoulate database with 100 entities
	for i := 0; i < 10; i++ {
		if _, err = db.Put("test-model", &worker.Model{}); err != nil {
			t.Fatalf("Failed to insert initial models: %v", err)
		}
	}

	// Run task in parallel
	parallel.Run(ctx, "test-model", 2, worker.Task)

	// Wait foreverrrr
	time.Sleep(10 * time.Second)

	// Check if our entities have been updated
	var models []worker.Model
	_, err = db.Query("test-model").GetAll(db.Context, &models)
	if err != nil {
		t.Fatalf("Unable to GetAll models: %v", err)
	}

	if len(models) != 10 {
		t.Fatalf("10 models not inserted into datastore: %v", len(models))
	}

	for _, model := range models {
		if model.Count != 1 {
			t.Fatalf("Model.Count is incorrect.")
		}
	}
}
