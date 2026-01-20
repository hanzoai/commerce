package tasks

import (
	"context"

	"github.com/gin-gonic/gin"

	"google.golang.org/appengine/memcache"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/util/task"
)

// SOON!
// task.Group("fixtures", func(){
// 	task.Register("products")
// 	task.Register("orders")
// })

var Foo = task.Func("foo", func(c *gin.Context) {
	foo := &memcache.Item{
		Key:   "foo",
		Value: []byte("bar"),
	}

	ctx := c.MustGet("appengine").(context.Context)
	if err := memcache.Set(ctx, foo); err != nil {
		log.Error(err, c)
	}
})

var Baz = task.Func("baz", func(c *gin.Context) {
	baz := &memcache.Item{
		Key:   "baz",
		Value: []byte("qux"),
	}

	ctx := c.MustGet("appengine").(context.Context)
	if err := memcache.Set(ctx, baz); err != nil {
		log.Error(err, c)
	}
})

var NestedBaz = task.Func("nested-baz", func(c *gin.Context) {
	ctx := c.MustGet("appengine").(context.Context)
	Baz.Call(ctx)
})
