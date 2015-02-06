package test

import (
	"path/filepath"
	"testing"
	"time"

	// "github.com/zeekay/aetest"
	"github.com/davidtai/appenginetesting"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/test/datastore/parallel/worker"
)

func checkCountValue(t *testing.T, models []worker.Model, v int) {
	for _, model := range models {
		if model.Count != v {
			t.Fatalf("Model.Count is %v, not %v.", model.Count, v)
		}
	}
}

func TestParallel(t *testing.T) {
	//Spin up an appengine dev server with the default module
	ctx, err := appenginetesting.NewContext(&appenginetesting.Options{
		AppId:      "crowdstart-io",
		Debug:      appenginetesting.LogChild,
		Testing:    t,
		TaskQueues: []string{"default"},
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
	time.Sleep(1 * time.Second)

	db := datastore.New(ctx)

	// Prepoulate database with 100 entities
	for i := 0; i < 10; i++ {
		model := &worker.Model{}
		if _, err := db.Put("test-model", model); err != nil {
			t.Fatalf("Failed to insert initial models: %v", err)
		}
	}

	// Run task in parallel
	parallel.Run(ctx, "test-model", 2, worker.TaskPlus1)

	time.Sleep(12 * time.Second)

	// Check if our entities have been updated
	var models []worker.Model
	_, err = db.Query("test-model").GetAll(db.Context, &models)
	if err != nil {
		t.Fatalf("Unable to GetAll models: %v", err)
	}

	if len(models) != 10 {
		t.Fatalf("10 models not inserted into datastore: %v", len(models))
	}

	checkCountValue(t, models, 1)
}

func TestParallelExtraParams(t *testing.T) {
	//Spin up an appengine dev server with the default module
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
	time.Sleep(1 * time.Second)

	db := datastore.New(ctx)

	// Prepoulate database with 100 entities
	for i := 0; i < 10; i++ {
		model := &worker.Model{}
		if _, err := db.Put("test-model", model); err != nil {
			t.Fatalf("Failed to insert initial models: %v", err)
		}
	}

	// Run task in parallel
	parallel.Run(ctx, "test-model", 2, worker.TaskSetVal, 100)

	time.Sleep(12 * time.Second)

	// Check if our entities have been updated
	var models []worker.Model
	_, err = db.Query("test-model").GetAll(db.Context, &models)
	if err != nil {
		t.Fatalf("Unable to GetAll models: %v", err)
	}

	if len(models) != 10 {
		t.Fatalf("10 models not inserted into datastore: %v", len(models))
	}

	checkCountValue(t, models, 100)
}
