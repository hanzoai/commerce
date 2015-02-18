package jobs

import (
	"reflect"

	"appengine"
	"appengine/delay"

	"crowdstart.io/thirdparty/salesforce"
	"crowdstart.io/util/log"
)

var jobs = make(map[string][]interface{})

// Add new job
func addJob(name string, fns ...interface{}) {
	// Create slice for job set
	if _, ok := jobs[name]; !ok {
		jobs[name] = make([]interface{}, 0)
	}

	// Append job
	for _, fn := range fns {
		jobs[name] = append(jobs[name], fn)
	}
}

// Run jobs
var Run = delay.Func("run-job", func(c appengine.Context, name string) {
	fns := jobs[name]
	for _, fn := range fns {
		switch v := fn.(type) {
		case *delay.Function:
			v.Call(c)
		case func(appengine.Context):
			v(c)
		default:
			log.Error("Couldn't execute %v", reflect.ValueOf(v).Type(), c)
		}
	}
})

// Define all jobs
func init() {
	addJob("import-users-to-salesforce", salesforce.ImportUsers)
	addJob("import-orders-to-salesforce", salesforce.ImportOrders)
	addJob("import-product-variants-to-salesforce", salesforce.ImportProductVariant)

	addJob("sync-salesforce", salesforce.CallPullUpdatedTask)
}
