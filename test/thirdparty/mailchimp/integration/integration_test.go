package mailchimp_integration_test

import (
	"testing"

	"hanzo.io/log"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/mailinglist"
	"hanzo.io/models/subscriber"
	"hanzo.io/thirdparty/mailchimp"
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
	ml  *mailinglist.MailingList
)

var _ = BeforeSuite(func() {
	var err error
	ctx = ae.NewContext()
	Expect(err).NotTo(HaveOccurred())

	// Mock gin context that we can use with fixtures
	c := gincontext.New(ctx)
	ml = fixtures.Mailinglist(c).(*mailinglist.MailingList)
})

var _ = AfterSuite(func() {
	err := ctx.Close()
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("ListSubscribe", func() {
	It("Should subscribe user to Mailchimp list", func() {
		sub := &subscriber.Subscriber{Email: "dev@hanzo.ai"}
		api := mailchimp.New(ctx, ml.Mailchimp.APIKey)
		err := api.Subscribe(ml, sub)
		Expect(err).NotTo(HaveOccurred())
	})
})
