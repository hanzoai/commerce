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
	Queue string
}

func (f *Function) Call(c appengine.Context, args ...interface{}) {
	t, _ := f.Task(args...)
	_, err := taskqueue.Add(c, t, f.Queue)
	if err != nil {
		log.Warn(err)
	}
}

func (f *Function) Task(args ...interface{}) (*taskqueue.Task, error) {
	return f.dfunc.Task(args...)
}

func Func(key string, i interface{}) *Function {
	fn := new(Function)
	fn.dfunc = delay.Func(key, i)
	fn.Queue = "" // Default queue
	return fn
}

func FuncUniq(key string, i interface{}) *Function {
	fn := new(Function)
	fn.dfunc = delay.Func(key, i)
	fn.Queue = key // Queue matches func name
	return fn
}
