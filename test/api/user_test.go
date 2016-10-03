package test

import (
	"crowdstart.com/models/user"
	"crowdstart.com/util/log"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("user", func() {
	Context("New user", func() {
		var req *user.User
		var res *user.User

		Before(func() {
			req = user.Fake(db)
			res = user.New(db)

			// Create new user
			log.JSON(req)
			cl.Post("/user", req, res)
			log.JSON(res)
		})

		It("Should create new users", func() {
			Expect(res.FirstName).To(Equal(req.FirstName))
			Expect(res.LastName).To(Equal(req.LastName))
			Expect(res.Username).To(Equal(req.Username))
			Expect(res.BillingAddress).To(Equal(req.BillingAddress))
			Expect(res.ShippingAddress).To(Equal(req.ShippingAddress))
			Expect(res.Email).To(Equal(req.Email))
			Expect(res.Enabled).To(Equal(req.Enabled))
		})
	})
})
