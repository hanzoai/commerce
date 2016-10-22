package test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
	"crowdstart.com/util/gincontext"
	"crowdstart.com/util/test/ae"
	"crowdstart.com/util/test/ginclient"

	stripeApi "crowdstart.com/thirdparty/stripe/api"

	. "crowdstart.com/util/test/ginkgo"
)

var (
	c   *gin.Context
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
