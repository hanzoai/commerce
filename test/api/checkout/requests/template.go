package requests

import "fmt"

type templateFunc func(...interface{}) string

func template(t string) templateFunc {
	return func(args ...interface{}) string {
		return fmt.Sprintf(t, args...)
	}
}

var t = template
