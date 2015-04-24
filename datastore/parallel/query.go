package parallel

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/util/fakecontext"
	"crowdstart.io/util/log"
)

// Wrap Query so we can do parallel jobs across custom query results
type Query struct {
	dq         *datastore.DatastoreQuery
	gincontext *gin.Context
	fn         *ParallelFn
}

func NewQuery(c *gin.Context, fn *ParallelFn) *Query {
	db := datastore.New(c)
	q := new(Query)
	q.gincontext = c
	q.dq = db.Query2(fn.Kind)
	q.fn = fn
	return q
}

func (q Query) RunAll(batchSize int, args ...interface{}) error {
	total, err := q.dq.Count()
	ctx := q.dq.Context

	if err != nil {
		log.Error("Count failed for %v: %v", q.fn.Kind, err, ctx)
		return err
	}

	namespace := ""
	maybeNamespace, err := q.gincontext.Get("namespace")
	if err == nil {
		namespace = maybeNamespace.(string)
	}

	// Limit results in test mode
	if q.gincontext.MustGet("test").(bool) {
		batchSize = 1
		total = 10
	}

	// Loop until all tasks have started with appropriate cursor, limit, offsets
	for offset := 0; offset < total; offset += batchSize {
		// Append variadic arguments after required args
		args := append([]interface{}{namespace, fakecontext.NewContext(q.gincontext), "", offset, batchSize}, args...)

		log.Debug("Namespace set %v", ctx, ctx)
		// Call delay.Function
		q.fn.DelayFn.Call(ctx, args...)
	}
	log.Warn("DONE")

	return nil
}
