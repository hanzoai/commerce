package mailchimp_integration_test

import (
	"testing"

	"github.com/zeekay/aetest"

	"crowdstart.com/util/log"

	. "crowdstart.com/util/test/ginkgo"
)

func Test(t *testing.T) {
	log.SetVerbose(testing.Verbose())
	Setup("thirdparty/mailchimp/integration", t)
}

var (
	ctx aetest.Context
)

var _ = BeforeSuite(func() {
	var err error
	ctx, err = aetest.NewContext(nil)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	err := ctx.Close()
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("ListSubscribe", func() {
	It("Should subscribe user to Mailchimp list", func() {
	})
})
