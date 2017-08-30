package test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/test/ae"

	"hanzo.io/thirdparty/stripe"
	. "hanzo.io/util/test/ginkgo"
)

var (
	c      *gin.Context
	ctx    ae.Context
	db     *datastore.Datastore
	client *stripe.Client
)

func Test(t *testing.T) {
	Setup("thirdparty/stripe", t)
}

var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	c = gincontext.New(ctx)
	db = datastore.New(c)
	client = stripe.New(ctx, "sk_test_RnnTXycI4vLympetwb66jTab")
})

var _ = AfterSuite(func() {
	ctx.Close()
})
