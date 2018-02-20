package delay

import (
	stdctx "context"
	"reflect"

	netctx "golang.org/x/net/context"
)

var (
	stdContextType = reflect.TypeOf((*stdctx.Context)(nil)).Elem()
	netContextType = reflect.TypeOf((*netctx.Context)(nil)).Elem()
)

func isContext(t reflect.Type) bool {
	return t == stdContextType || t == netContextType
}
