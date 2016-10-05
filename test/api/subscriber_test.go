package test

import (
	"crowdstart.com/models/subscriber"
	"crowdstart.com/models/user"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("subscriber", func() {
	Context("New subscriber", func() {
		req := new(subscriber.Subscriber)
		res := new(subscriber.Subscriber)

		Before(func() {
			usr := user.Fake(db)
			usr.MustCreate()
			req = subscriber.Fake(db, usr.Id())
			res = subscriber.New(db)

			// Create new subscriber
			cl.Post("/subscriber", req, res)
		})

		It("Should create new subscribers", func() {
			Expect(res.Email).To(Equal(req.Email))
			Expect(res.UserId).To(Equal(req.UserId))
			Expect(res.MailingListId).To(Equal(req.MailingListId))
			Expect(res.Unsubscribed).To(Equal(req.Unsubscribed))
		})
	})
	Context("Get subscriber", func() {
		req := new(subscriber.Subscriber)
		res := new(subscriber.Subscriber)

		Before(func() {
			usr := user.Fake(db)
			usr.MustCreate()
			req = subscriber.Fake(db, usr.Id())
			req.MustCreate()

			res = subscriber.New(db)

			cl.Get("/subscriber/"+req.Id(), res)
		})

		It("Should create new subscribers", func() {
			Expect(res.Email).To(Equal(req.Email))
			Expect(res.UserId).To(Equal(req.UserId))
			Expect(res.MailingListId).To(Equal(req.MailingListId))
			Expect(res.Unsubscribed).To(Equal(req.Unsubscribed))
		})
	})
	Context("Delete subscriber", func() {
		res := ""

		Before(func() {
			usr := user.Fake(db)
			usr.MustCreate()
			req := subscriber.Fake(db, usr.Id())
			req.MustCreate()

			cl.Delete("/subscriber/" + req.Id())
			res = req.Id()
		})

		It("Should create new subscribers", func() {
			sub := subscriber.New(db)
			err := sub.GetById(res)
			Expect(err).ToNot(BeNil())
		})
	})
})
