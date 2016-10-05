package test

import (
	"crowdstart.com/models/webhook"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("webhook", func() {
	Context("New webhook", func() {
		req := new(webhook.Webhook)
		res := new(webhook.Webhook)

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
	Context("Get webhook", func() {
		req := new(webhook.Webhook)
		res := new(webhook.Webhook)

		Before(func() {
			req = webhook.Fake(db)
			req.MustCreate()

			res = webhook.New(db)

			cl.Get("/webhook/"+req.Id(), res)
		})

		It("Should get webhooks", func() {
			Expect(res.Enabled).To(Equal(req.Enabled))
			Expect(res.Url).To(Equal(req.Url))
			Expect(res.Live).To(Equal(req.Live))
			Expect(res.All).To(Equal(req.All))
		})
	})
	Context("Delete webhook", func() {
		res := ""

		Before(func() {
			req := webhook.Fake(db)
			req.MustCreate()

			cl.Delete("/webhook/" + req.Id())
			res = req.Id()
		})

		It("Should delete webhooks", func() {
			hook := webhook.New(db)
			err := hook.GetById(res)
			Expect(err).ToNot(BeNil())
		})
	})
})
