package test

import (
	"strings"

	"crowdstart.com/models/user"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = FDescribe("user", func() {
	var normalize = func(s string) string {
		return strings.ToLower(strings.TrimSpace(s))
	}

	Context("New user", func() {
		req := new(user.User)
		res := new(user.User)

		Before(func() {
			req = user.Fake(db)
			res = user.New(db)

			cl.Post("/user", req, res)
		})

		It("Should create new users", func() {
			Expect(res.FirstName).To(Equal(req.FirstName))
			Expect(res.LastName).To(Equal(req.LastName))
			Expect(res.Username).To(Equal(normalize(req.Username)))
			Expect(res.BillingAddress).To(Equal(req.BillingAddress))
			Expect(res.ShippingAddress).To(Equal(req.ShippingAddress))
			Expect(res.Email).To(Equal(normalize(req.Email)))
			Expect(res.Enabled).To(Equal(req.Enabled))
		})
	})

	Context("Get user", func() {
		req := new(user.User)
		res := new(user.User)

		Before(func() {
			req = user.Fake(db)
			req.MustCreate()

			res = user.New(db)

			cl.Get("/user/"+req.Id(), res)
		})

		It("Should create new users", func() {
			Expect(res.FirstName).To(Equal(req.FirstName))
			Expect(res.LastName).To(Equal(req.LastName))
			Expect(res.Username).To(Equal(normalize(req.Username)))
			Expect(res.BillingAddress).To(Equal(req.BillingAddress))
			Expect(res.ShippingAddress).To(Equal(req.ShippingAddress))
			Expect(res.Email).To(Equal(normalize(req.Email)))
			Expect(res.Enabled).To(Equal(req.Enabled))
		})
	})

	Context("Delete user", func() {
		res := ""

		Before(func() {
			req := user.Fake(db)
			req.MustCreate()

			cl.Delete("/user/" + req.Id())
			res = req.Id()
		})

		It("Should create new users", func() {
			usr := user.New(db)
			err := usr.GetById(res)
			Expect(err).ToNot(BeNil())
		})
	})
})
