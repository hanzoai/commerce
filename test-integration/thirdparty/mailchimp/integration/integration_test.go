package mailchimp_integration_test

import (
	"testing"

	"github.com/hanzoai/commerce/log"
	// "github.com/hanzoai/commerce/models/subscriber"
	"github.com/hanzoai/commerce/thirdparty/mailchimp"
	"github.com/hanzoai/commerce/types/email"
	"github.com/hanzoai/commerce/types/integration"
	"github.com/hanzoai/commerce/util/gincontext"
	"github.com/hanzoai/commerce/util/test/ae"

	. "github.com/hanzoai/commerce/util/test/ginkgo"
)

func Test(t *testing.T) {
	log.SetVerbose(testing.Verbose())
	Setup("thirdparty/mailchimp/integration", t)
}

var (
	ctx ae.Context
)

var _ = BeforeSuite(func() {
	var err error
	ctx = ae.NewContext()
	Expect(err).NotTo(HaveOccurred())

	// Mock gin context that we can use with fixtures
	_ = gincontext.New(ctx)
})

var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("ListSubscribe", func() {
	It("Should subscribe user to Mailchimp list", func() {
		// sub := &subscriber.Subscriber{Email: "dev@hanzo.ai"}

		l := new(email.List)
		sub := new(email.Subscriber)
		setting := integration.Mailchimp{ListId: "421751eb03", APIKey: "", CheckoutUrl: ""}
		api := mailchimp.New(ctx, setting)
		err := api.Subscribe(l, sub)
		Expect(err).NotTo(HaveOccurred())
	})
})
