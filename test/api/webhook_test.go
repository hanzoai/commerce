package test

import (
	"crowdstart.com/models/webhook"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("webhook", func() {
	Context("New webhook", func() {
		var req *webhook.Webhook
		var res *webhook.Webhook

		Before(func() {
			req = webhook.Fake(db)
			res = webhook.New(db)

			// Create new webhook
			cl.Post("/webhook", req, res)
		})

		It("Should create new webhooks", func() {
			Expect(res.Enabled).To(Equal(req.Enabled))
			Expect(res.Url).To(Equal(req.Url))
			Expect(res.Live).To(Equal(req.Live))
			Expect(res.All).To(Equal(req.All))
		})
	})
})
