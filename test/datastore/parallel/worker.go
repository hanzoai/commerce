package test

// Define a new worker with parallel.Task
var TestWorker = parallel.Task("test-worker", func(db *datastore.Datastore, k datastore.Key, model TestCounter) {
	model.Count = model.Count + 1
	db.PutKey("test-counter", k, model)
})
