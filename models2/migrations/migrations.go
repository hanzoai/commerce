package migration

import (
	"reflect"
	"strconv"

	"appengine/delay"

	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore/parallel"
	"crowdstart.io/util/log"
	"crowdstart.io/util/task"
)

type SetupFn func(*gin.Context)

func New(name string, setupFn SetupFn, fns ...interface{}) *delay.Function {
	name = "migration-" + name

	tasks := make([]*parallel.ParallelFn, len(fns))
	for i, fn := range fns {
		tasks[i] = parallel.New(name+"-task-"+strconv.Itoa(i), fn)
	}

	return task.Func(name, func(c *gin.Context) {
		for i, fn := range fns {
			// Check type of worker func to ensure it matches required signature.
			t := reflect.TypeOf(fn)

			// Ensure that workerFunc is actually a func
			if t.Kind() != reflect.Func {
				log.Panic("Non-function passed in as migration.")
			}

			argNum := t.NumIn()
			if argNum < 3 {
				log.Panic("Function requires at least three arguments")
			}

			tasks[i].Run(c, 50)
		}
	})
}
