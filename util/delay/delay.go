// derived from https://github.com/golang/appengine/blob/75a29a66d4850a15c19eb6d70a31f5c453572be0/delay/delay.go
package delay

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"hanzo.io/util/log"

	"google.golang.org/appengine"
	"google.golang.org/appengine/taskqueue"
)

const (
	// The HTTP path for invocations.
	path = "/hanzo_task/queue/go/delay"
	// Use the default queue.
	queue = ""
)

var (
	// registry of all delayed functions
	Funcs = make(map[string]*Function)

	// precomputed types
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
	errorType   = reflect.TypeOf((*error)(nil)).Elem()

	// errors
	errFirstArg = errors.New("first argument must be context.Context")
)

// Simple wrapper around delay.Func which allows queue to be customized
type Function struct {
	fv  reflect.Value // Kind() == reflect.Func
	key string
	err error // any error during initialization

	queue string
	name  string
	delay time.Duration
}

type invocation struct {
	Key  string
	Args []interface{}
}

// Func declares a new Function. The second argument must be a function with a
// first argument of type context.Context.
// This function must be called at program initialization time. That means it
// must be called in a global variable declaration or from an init function.
// This restriction is necessary because the instance that delays a function
// call may not be the one that executes it. Only the code executed at program
// initialization time is guaranteed to have been run by an instance before it
// receives a request.
func Func(key string, i interface{}) *Function {
	f := &Function{fv: reflect.ValueOf(i)}

	f.key = key

	t := f.fv.Type()
	if t.Kind() != reflect.Func {
		f.err = errors.New("not a function")
		return f
	}
	if t.NumIn() == 0 || t.In(0) != contextType {
		f.err = errFirstArg
		return f
	}

	// Register the function's arguments with the gob package.
	// This is required because they are marshaled inside a []interface{}.
	// gob.Register only expects to be called during initialization;
	// that's fine because this function expects the same.
	for i := 0; i < t.NumIn(); i++ {
		// Only concrete types may be registered. If the argument has
		// interface type, the client is resposible for registering the
		// concrete types it will hold.
		if t.In(i).Kind() == reflect.Interface {
			continue
		}
		gob.Register(reflect.Zero(t.In(i)).Interface())
	}

	if old := Funcs[f.key]; old != nil {
		old.err = fmt.Errorf("multiple functions registered for %s", key)
	}
	Funcs[f.key] = f

	f.queue = "" // Use default queue
	f.name = ""  // Use taskqueue-generated name
	f.delay = 0
	return f
}

// Call invokes a delayed function.
//   err := f.Call(c, ...)
// is equivalent to
//   t, _ := f.Task(...)
//   _, err := taskqueue.Add(c, t, "")
func (f *Function) Call(c context.Context, args ...interface{}) error {
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
	if _, err := taskqueue.Add(c, t, f.queue); err != nil {
		log.Warn(err)
		return err
	}

	return nil
}

// Task creates a Task that will invoke the function.
// Its parameters may be tweaked before adding it to a queue.
// Users should not modify the Path or Payload fields of the returned Task.
func (f *Function) Task(args ...interface{}) (*taskqueue.Task, error) {
	if f.err != nil {
		return nil, fmt.Errorf("delay: func is invalid: %v", f.err)
	}

	nArgs := len(args) + 1 // +1 for the context.Context
	ft := f.fv.Type()
	minArgs := ft.NumIn()
	if ft.IsVariadic() {
		minArgs--
	}
	if nArgs < minArgs {
		return nil, fmt.Errorf("delay: too few arguments to func: %d < %d", nArgs, minArgs)
	}
	if !ft.IsVariadic() && nArgs > minArgs {
		return nil, fmt.Errorf("delay: too many arguments to func: %d > %d", nArgs, minArgs)
	}

	// Check arg types.
	for i := 1; i < nArgs; i++ {
		at := reflect.TypeOf(args[i-1])
		var dt reflect.Type
		if i < minArgs {
			// not a variadic arg
			dt = ft.In(i)
		} else {
			// a variadic arg
			dt = ft.In(minArgs).Elem()
		}
		// nil arguments won't have a type, so they need special handling.
		if at == nil {
			// nil interface
			switch dt.Kind() {
			case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
				continue // may be nil
			}
			return nil, fmt.Errorf("delay: argument %d has wrong type: %v is not nilable", i, dt)
		}
		switch at.Kind() {
		case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
			av := reflect.ValueOf(args[i-1])
			if av.IsNil() {
				// nil value in interface; not supported by gob, so we replace it
				// with a nil interface value
				args[i-1] = nil
			}
		}
		if !at.AssignableTo(dt) {
			return nil, fmt.Errorf("delay: argument %d has wrong type: %v is not assignable to %v", i, at, dt)
		}
	}

	inv := invocation{
		Key:  f.key,
		Args: args,
	}

	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(inv); err != nil {
		return nil, fmt.Errorf("delay: gob encoding failed: %v", err)
	}

	return &taskqueue.Task{
		Path:    path,
		Payload: buf.Bytes(),
	}, nil
}

// Returns a copy of this Function with new queue settings
func (f *Function) Queue(queue string) *Function {
	f2 := new(Function)
	f2.queue = queue
	return f2
}

// Add a task only once by using a unique name
func (f *Function) Once(ctx context.Context, name string, delay time.Duration, args ...interface{}) error {
	f2 := *f
	f2.queue = f.queue
	f2.name = name
	f2.delay = delay
	return f2.Call(ctx, args...)
}

func FuncByKey(key string) *Function {
	f, ok := Funcs[key]
	if !ok {
		keys := []string{}
		for k := range Funcs {
			keys = append(keys, k)
		}

		panic(fmt.Errorf("delay: key %s not found in delay.Funcs(%s)", key, keys))
	}
	return f
}

func init() {
	http.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		runFunc(appengine.NewContext(req), w, req)
	})
}

func runFunc(c context.Context, w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	var inv invocation
	if err := gob.NewDecoder(req.Body).Decode(&inv); err != nil {
		log.Error(c, "delay: failed decoding task payload: %v", err)
		log.Warn(c, "delay: dropping task")
		return
	}

	f := Funcs[inv.Key]
	if f == nil {
		log.Error(c, "delay: no func with key %q found", inv.Key)
		log.Warn(c, "delay: dropping task")
		return
	}

	ft := f.fv.Type()
	in := []reflect.Value{reflect.ValueOf(c)}
	for _, arg := range inv.Args {
		var v reflect.Value
		if arg != nil {
			v = reflect.ValueOf(arg)
		} else {
			// Task was passed a nil argument, so we must construct
			// the zero value for the argument here.
			n := len(in) // we're constructing the nth argument
			var at reflect.Type
			if !ft.IsVariadic() || n < ft.NumIn()-1 {
				at = ft.In(n)
			} else {
				at = ft.In(ft.NumIn() - 1).Elem()
			}
			v = reflect.Zero(at)
		}
		in = append(in, v)
	}
	out := f.fv.Call(in)

	if n := ft.NumOut(); n > 0 && ft.Out(n-1) == errorType {
		if errv := out[n-1]; !errv.IsNil() {
			log.Error(c, "delay: func failed (will retry): %v", errv.Interface())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}
