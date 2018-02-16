package test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/test/ae"
	"hanzo.io/util/test/ginclient"

	stripeApi "hanzo.io/thirdparty/stripe/api"

	. "hanzo.io/util/test/ginkgo"
)

var (
	c   *context.Context
	ctx ae.Context
	cl  *ginclient.Client
	db  *datastore.Datastore
	org *organization.Organization
)

func Test(t *testing.T) {
	Setup("thirdparty/stripe/webhook", t)
}

var _ = BeforeSuite(func() {
	// Create new context
	ctx = ae.NewContext(ae.Options{
		Modules:    []string{"default"},
		TaskQueues: []string{"default"},
		Noisy:      testing.Verbose(),
	})

	// Get reference to datastore
	c = gincontext.New(ctx)
	db = datastore.New(c)

	// Setup organization
	org = organization.New(db)
	org.Stripe.UserId = "1"
	org.Stripe.Test.UserId = "1"
	org.MustCreate()

	// Create gin client
	cl = ginclient.New(ctx)

	// Add stripe routes
	stripeApi.Route(cl.Router)
})

var _ = AfterSuite(func() {
	ctx.Close()
})
