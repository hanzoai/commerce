package tasks

import (
	"github.com/gin-gonic/gin"

	"appengine"
	"appengine/memcache"

	"crowdstart.io/util/log"
	"crowdstart.io/util/task"
)

var Foo = task.Func("foo", func(c *gin.Context) {
	foo := &memcache.Item{
		Key:   "foo",
		Value: []byte("bar"),
	}

	ctx := c.MustGet("appengine").(appengine.Context)
	if err := memcache.Set(ctx, foo); err != nil {
		log.Error(err, c)
	}
})
