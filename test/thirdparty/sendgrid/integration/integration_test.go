package mandrill_integration_test

import (
	"testing"

	"hanzo.io/config"
	"hanzo.io/log"
	"hanzo.io/thirdparty/sendgrid"
	"hanzo.io/types/email"
	"hanzo.io/types/integration"
	"hanzo.io/util/test/ae"
	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	log.SetVerbose(testing.Verbose())
	Setup("thirdparty/sendgrid/integration", t)
}

var (
	ctx      ae.Context
	settings = integration.SendGrid{
		APIKey: config.SendGrid.APIKey,
	}
)

var _ = BeforeSuite(func() {
	var err error
	ctx = ae.NewContext()
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("Send", func() {
	if config.SendGrid.APIKey == "" {
		return
	}

	It("Should send email", func() {
		client := sendgrid.New(ctx, settings)
		message := email.NewMessage()
		message.From = email.Email{
			Name:    "Hanzo",
			Address: "test@hanzo.ai",
		}
		message.AddTos(email.Email{
			Name:    "Hanzo Test",
			Address: "relay@hanzo.ai",
		})
		message.Subject = "Test"
		message.HTML = `
		<html>
		hi!
		</html>
		`
		message.Text = `
		Hi!
		`

		err := client.Send(message)
		Expect(err).NotTo(HaveOccurred())
	})
})
