package migrations

import (
	"strconv"

	"appengine/delay"

	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore/parallel"
	"crowdstart.io/util/task"
)

type SetupFn func(*gin.Context)

func NoSetup(c *gin.Context) {
}

func New(name string, setupFn SetupFn, fns ...interface{}) *delay.Function {
	name = "migration-" + name

	tasks := make([]*parallel.ParallelFn, len(fns))
	for i, fn := range fns {
		tasks[i] = parallel.New(name+"-task-"+strconv.Itoa(i), fn)
	}

	return task.Func(name, func(c *gin.Context) {
		// Call setup fn
		setupFn(c)

		for i, _ := range fns {
			// Run task fn
			tasks[i].Run(c, 50)
		}
	})
}
