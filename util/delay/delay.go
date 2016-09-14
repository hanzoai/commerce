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
}

// Create a new Function assigned to default queue
func Func(key string, i interface{}) *Function {
	fn := new(Function)
	fn.dfunc = delay.Func(key, i)
	fn.queue = "" // Use default queue
	return fn
}

// Wrapper around delay.Func.Call
func (f *Function) Call(c appengine.Context, args ...interface{}) {
	t, _ := f.Task(args...)
	_, err := taskqueue.Add(c, t, f.queue)
	if err != nil {
		log.Warn(err)
	}
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
