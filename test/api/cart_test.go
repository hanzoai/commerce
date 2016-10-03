package test

import (
	"crowdstart.com/models/cart"
	"crowdstart.com/models/user"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("cart", func() {
	Context("New cart", func() {
		var req *cart.Cart
		var res *cart.Cart

		Before(func() {
			usr := user.Fake(db)
			usr.MustCreate()
			req = cart.Fake(db, usr.Id())
			res = cart.New(db)

			// Create new cart
			cl.Post("/cart", req, res)
		})

		It("Should create new carts", func() {
			Expect(res.Name).To(Equal(req.Name))
			Expect(res.Company).To(Equal(req.Company))
			Expect(res.Country).To(Equal(req.Country))
			Expect(res.TaxId).To(Equal(req.TaxId))
			Expect(res.Commission.Flat).To(Equal(req.Commission.Flat))
			Expect(res.Commission.Minimum).To(Equal(req.Commission.Minimum))
			Expect(res.Commission.Percent).To(Equal(req.Commission.Percent))
		})
	})
})
