package delay

import (
	"context"
	"encoding/gob"
	"net/http"
	"reflect"

	"google.golang.org/appengine/taskqueue"

	"github.com/hanzoai/commerce/log"
)

func RunFunc(c context.Context, w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	c = context.WithValue(c, headersContextKey, taskqueue.ParseRequestHeaders(req.Header))

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
