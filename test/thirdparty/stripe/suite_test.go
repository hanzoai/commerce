package test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/gincontext"
	"crowdstart.io/util/queries"
	"crowdstart.io/util/test/ae"

	. "crowdstart.io/util/test/ginkgo"
)

var (
	c        *gin.Context
	ctx      ae.Context
	db       *datastore.Datastore
	q        *queries.Client
	campaign models.Campaign
)

func Test(t *testing.T) {
	Setup("thirdparty/stripe", t)
}

var _ = BeforeSuite(func() {
	// gob.Register(models.Campaign{})
	ctx = ae.NewContext(ae.Options{
		Modules:    []string{"default"},
		TaskQueues: []string{"default"},
	})

	db = datastore.New(ctx)
	q = queries.New(ctx)
	c = gincontext.New(ctx)

	campaign.Id = "dev@hanzo.ai"
	campaign.Creator.Email = campaign.Id
	campaign.Stripe.UserId = "acct_something"
	campaign.Stripe.Livemode = false
	campaign.Stripe.AccessToken = config.Stripe.APISecret
})

var _ = AfterSuite(func() {
	ctx.Close()
})
