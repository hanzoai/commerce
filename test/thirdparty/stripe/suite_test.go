package test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/organization"
	"hanzo.io/util/gincontext"
	"hanzo.io/log"
	"hanzo.io/util/test/ae"

	"hanzo.io/thirdparty/stripe"
	. "hanzo.io/util/test/ginkgo"
)

var (
	c      *gin.Context
	ctx    ae.Context
	db     *datastore.Datastore
	client *stripe.Client
	token string
)

func Test(t *testing.T) {
	Setup("thirdparty/stripe", t)
}

var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	c = gincontext.New(ctx)
	db = datastore.New(c)
	log.Warn("Before Suite")
	org := fixtures.Organization(c).(*organization.Organization)
	token = org.Stripe.Test.AccessToken
	client = stripe.New(ctx, token)
})

var _ = AfterSuite(func() {
	ctx.Close()
})
