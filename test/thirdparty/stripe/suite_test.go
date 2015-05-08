package test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/util/test/ae"

	. "crowdstart.com/util/test/ginkgo"
)

var (
	c   *gin.Context
	ctx ae.Context
	db  *datastore.Datastore
	// campaign models.Campaign
)

func Test(t *testing.T) {
	Setup("thirdparty/stripe", t)
}

var _ = BeforeSuite(func() {
	// // gob.Register(models.Campaign{})
	// ctx = ae.NewContext(ae.Options{
	// 	Modules:    []string{"default"},
	// 	TaskQueues: []string{"default"},
	// })

	// db = datastore.New(ctx)
	// q = queries.New(ctx)
	// c = gincontext.New(ctx)

	// campaign.Id = "dev@hanzo.ai"
	// campaign.Creator.Email = campaign.Id
	// campaign.Stripe.UserId = "acct_something"
	// campaign.Stripe.Livemode = false
	// campaign.Stripe.AccessToken = config.Stripe.SecretKey
})

var _ = AfterSuite(func() {
	// ctx.Close()
})
