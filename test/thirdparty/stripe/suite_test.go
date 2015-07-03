package test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/util/gincontext"
	"crowdstart.com/util/test/ae"

	. "crowdstart.com/util/test/ginkgo"
)

var (
	c   *gin.Context
	ctx ae.Context
	db  *datastore.Datastore
)

func Test(t *testing.T) {
	Setup("thirdparty/stripe", t)
}

var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	c = gincontext.New(ctx)
	db = datastore.New(c)
})

var _ = AfterSuite(func() {
	ctx.Close()
})
