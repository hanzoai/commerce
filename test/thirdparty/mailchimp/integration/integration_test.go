package mailchimp_integration_test

import (
	"testing"

	"hanzo.io/log"
	"hanzo.io/models/fixtures"
	"hanzo.io/thirdparty/mailchimp"
	"hanzo.io/types/email"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/test/ae"

	. "hanzo.io/util/test/ginkgo"
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
	c := gincontext.New(ctx)
})

var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("ListSubscribe", func() {
	It("Should subscribe user to Mailchimp list", func() {
		sub := &subscriber.Subscriber{Email: "dev@hanzo.ai"}
		api := mailchimp.New(ctx, ml.Mailchimp.APIKey)

		l := new(email.List)
		sub := new(email.Subscriber)
		err := api.Subscribe(l, sub)
		Expect(err).NotTo(HaveOccurred())
	})
})
