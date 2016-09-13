package task

import (
	"reflect"
	"sort"
	"strconv"

	"appengine"
	"appengine/delay"

	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/util/fakecontext"
	"crowdstart.com/util/gincontext"
	"crowdstart.com/util/log"
)

var (
	Registry    = make(map[string][]interface{})
	contextType = reflect.TypeOf((**gin.Context)(nil)).Elem()
)

type Task struct {
	ExpectsQuery bool
	Fn           interface{}
	Value        reflect.Value
	DelayFn      *delay.Function
}

func New(name string, fn interface{}) *Task {
	t := NewTask(fn)
	Register(name, t)
	return t
}

func NewTask(fn interface{}) *Task {
	task := new(Task)

	// Check type of worker func to ensure it matches required signature.
	t := reflect.TypeOf(fn)

	// Ensure that fn is actually a func
	if t.Kind() != reflect.Func {
		log.Panic("Function is required for second parameter")
	}

	// fn should be a function that takes at least three arguments
	argNum := t.NumIn()
	if argNum < 1 {
		log.Panic("Function requires at least one argument")
	}

	// First argument fn should be gin.Context
	if t.In(0) != contextType {
		log.Panic("First argument must be *gin.Context: %v", t)
	}

	// Get reflect.Value of fn so we can call from delay.Func
	task.Value = reflect.ValueOf(fn)
	return task
}

// Register a new task
func Register(name string, tasks ...interface{}) int {
	// Create slice for task set
	_tasks, ok := Registry[name]

	if !ok {
		_tasks = make([]interface{}, 0)
	}

	// Append tasks
	Registry[name] = append(_tasks, tasks...)

	return len(Registry[name])
}

// Remove tasks registered under a given name.
func Unregister(name string) {
	delete(Registry, name)
}

// Returns a slice of task names
func Names() []string {
	tasks := make([]string, 0)
	for k, _ := range Registry {
		tasks = append(tasks, k)
	}
	sort.Strings(tasks)
	return tasks
}

// Run task
func Run(ctx *gin.Context, name string, args ...interface{}) {
	tasks, ok := Registry[name]

	if !ok {
		log.Panic("Unknown task: %v", name, ctx)
	}

	for i := 0; i < len(tasks); i++ {
		switch v := tasks[i].(type) {
		case *delay.Function:
			v.Call(middleware.GetAppEngine(ctx), args...)
		case func(appengine.Context):
			v(middleware.GetAppEngine(ctx)) // TODO: Remove after updating older tasks.
		case func(appengine.Context, ...interface{}):
			v(middleware.GetAppEngine(ctx), args...)
		case func(*gin.Context):
			v(ctx)
		case func(*gin.Context, ...interface{}):
			v(ctx, args...)
		case *Task:
			v.DelayFn.Call(middleware.GetAppEngine(ctx), fakecontext.NewContext(ctx))
		default:
			log.Panic("Don't know how to call %v", reflect.ValueOf(v).Type(), ctx)
		}
	}
}

func getGinContext(ctx appengine.Context, fakectx *fakecontext.Context, ok bool) *gin.Context {
	// If we have a fake context, try to use that
	if ok {
		if c, err := fakectx.Context(&ctx); err == nil {
			return c
		}
	}

	return gincontext.New(ctx)
}

// Creates a new delay.Func which will call our fn with gin.Context, etc.
func Func(name string, fn interface{}) *delay.Function {
	task := NewTask(fn)

	// Automatically register task
	n := Register(name, task)

	// Increment name for delayFn if this is a duplicate func
	if n > 1 {
		name = name + "-" + strconv.Itoa(n)
	}

	// Create actual delay func
	delayFn := delay.Func(name, func(c appengine.Context, args ...interface{}) {
		log.Debug("Args: %#v", args, c)
		// Try to retrieve fake context from args
		var fakectx *fakecontext.Context
		var ok bool

		// Check if we were passed a fakecontext
		if len(args) > 0 {
			fakectx, ok = args[0].(*fakecontext.Context)
		}

		// Remove fakecontext from args, if it exists
		if ok {
			args = args[:len(args)-1]
		}

		// Recreate gin context from fakecontext if possible, otherwise
		// create a new one using this appengine context.
		ctx := getGinContext(c, fakectx, ok)

		// Build arguments for fn
		in := []reflect.Value{reflect.ValueOf(ctx)}

		// Append variadic args
		for _, arg := range args {
			in = append(in, reflect.ValueOf(arg))
		}

		// Call fn
		task.Value.Call(in)
	})

	// Save reference to delay function for easy access from Task
	task.DelayFn = delayFn

	return delayFn
}
