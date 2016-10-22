package delay

import (
	"crowdstart.com/util/log"

	"appengine"
	"appengine/delay"
	"appengine/taskqueue"
)

// Simple wrapper around delay.Func which allows queue to be customized
type Function struct {
	dfunc *delay.Function
	queue string
	name  string
}

// Create a new Function assigned to default queue
func Func(key string, i interface{}) *Function {
	fn := new(Function)
	fn.dfunc = delay.Func(key, i)
	fn.queue = "" // Use default queue
	return fn
}

// Wrapper around delay.Func.Call
func (f *Function) Call(ctx appengine.Context, args ...interface{}) error {
	// Get task from delay.Func
	t, err := f.Task(args...)
	if err != nil {
		log.Warn(err)
		return err
	}

	// Override name
	if f.name != "" {
		t.Name = f.name
	}

	// Add to taskqueue
	if _, err := taskqueue.Add(ctx, t, f.queue); err != nil {
		log.Warn(err)
		return err
	}

	return nil
}

// Wrapper around delay.Func.Task
func (f *Function) Task(args ...interface{}) (*taskqueue.Task, error) {
	return f.dfunc.Task(args...)
}

// Returns a copy of this Function with new queue settings
func (f *Function) Queue(queue string) *Function {
	f2 := new(Function)
	f2.dfunc = f.dfunc
	f2.queue = queue
	return f2
}

// Add a task only once by using a unique name
func (f *Function) Once(ctx appengine.Context, name string, args ...interface{}) error {
	f2 := new(Function)
	f2.dfunc = f.dfunc
	f2.queue = f.queue
	f2.name = name
	return f2.Call(ctx, args...)
}
