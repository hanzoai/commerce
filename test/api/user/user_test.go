package test

import (
	"strings"

	"github.com/icrowley/fake"

	"crowdstart.com/models/user"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("user", func() {
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

	Context("Patch user", func() {
		usr := new(user.User)
		res := new(user.User)

		req := struct {
			FirstName string `json:"firstName"`
			LastName  string `json:"lastName"`
		}{
			fake.FirstName(),
			fake.LastName(),
		}

		Before(func() {
			// Create user
			usr = user.Fake(db)
			usr.MustCreate()

			// Patch user
			cl.Patch("/user/"+usr.Id(), req, res)
		})

		It("Should patch user", func() {
			Expect(res.Id_).To(Equal(usr.Id()))

			Expect(res.FirstName).To(Equal(req.FirstName))
			Expect(res.LastName).To(Equal(req.LastName))

			Expect(res.Username).To(Equal(normalize(usr.Username)))
			Expect(res.BillingAddress).To(Equal(usr.BillingAddress))
			Expect(res.ShippingAddress).To(Equal(usr.ShippingAddress))
			Expect(res.Email).To(Equal(normalize(usr.Email)))
			Expect(res.Enabled).To(Equal(usr.Enabled))
		})
	})

	Context("Put user", func() {
		usr := new(user.User)
		res := new(user.User)
		req := new(user.User)

		Before(func() {
			// Create user
			usr = user.Fake(db)
			usr.MustCreate()

			// Create user request
			req = user.Fake(db)

			// Update user
			cl.Put("/user/"+usr.Id(), req, res)
		})

		It("Should put user", func() {
			Expect(res.Id_).To(Equal(usr.Id()))
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
