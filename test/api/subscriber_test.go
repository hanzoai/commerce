package test

import (
	"crowdstart.com/models/subscriber"
	"crowdstart.com/models/user"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("subscriber", func() {
	Context("New subscriber", func() {
		var req *subscriber.Subscriber
		var res *subscriber.Subscriber

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
})
