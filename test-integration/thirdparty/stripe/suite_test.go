package test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/fixtures"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/gincontext"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/util/test/ae"

	"github.com/hanzoai/commerce/thirdparty/stripe"
	. "github.com/hanzoai/commerce/util/test/ginkgo"
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
