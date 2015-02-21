package mandrill_integration_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"appengine"

	"github.com/zeekay/aetest"

	"crowdstart.io/config"
	"crowdstart.io/thirdparty/mandrill"
	"crowdstart.io/util/log"
)

func Test(t *testing.T) {
	log.SetVerbose(testing.Verbose())
	RegisterFailHandler(Fail)
	RunSpecs(t, "thirdparty/mandrill/integration")
}

var (
	instance aetest.Instance
	ctx      appengine.Context
)

var _ = BeforeSuite(func() {
	if config.Mandrill.APIKey == "" {
		GinkgoT().Skip()
	}

	instance, err := aetest.NewInstance(nil)
	Expect(err).NotTo(HaveOccurred())

	req, err := instance.NewRequest("", "", nil)
	Expect(err).NotTo(HaveOccurred())

	ctx = appengine.NewContext(req)
})

var _ = AfterSuite(func() {
	err := instance.Close()
	Expect(err).NotTo(HaveOccurred())

	err = instance.Close()
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("Ping", func() {
	Expect(mandrill.Ping(ctx)).To(Equal(true))
})

var _ = Describe("Send", func() {
	html := mandrill.GetTemplate("../templates/confirmation_email.html")
	req := mandrill.NewSendReq()
	req.AddRecipient("dev@hanzo.ai", "Test Mandrill")

	req.Message.Subject = "Test subject"
	req.Message.FromEmail = "dev@hanzo.ai"
	req.Message.FromName = "Tester"
	req.Message.Html = html

	err := mandrill.Send(ctx, &req)
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("SendTemplate", func() {
	req := mandrill.NewSendTemplateReq()
	// req.AddRecipient("dev@hanzo.ai", "Zach Kelling")
	// req.AddRecipient("dev@hanzo.ai", "Michael W")
	// req.AddRecipient("dev@hanzo.ai", "Marvel Mathew")
	// req.AddRecipient("dev@hanzo.ai", "David Tai")
	req.AddRecipient("dev@hanzo.ai", "Test Mandrill")

	req.Message.Subject = "Test subject"
	req.Message.FromEmail = "dev@hanzo.ai"
	req.Message.FromName = "Tester"
	req.TemplateName = "preorder-confirmation-template"

	err := mandrill.SendTemplate(ctx, &req)
	Expect(err).NotTo(HaveOccurred())
})
