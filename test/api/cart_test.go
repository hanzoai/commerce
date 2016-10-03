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
			Expect(res.UserId).To(Equal(req.UserId))
			Expect(res.Email).To(Equal(req.Email))
			Expect(res.ReferrerId).To(Equal(req.ReferrerId))
			Expect(res.Status).To(Equal(req.Status))
			Expect(res.Currency).To(Equal(req.Currency))
			Expect(res.LineTotal).To(Equal(req.LineTotal))
			Expect(res.Discount).To(Equal(req.Discount))
			Expect(res.Subtotal).To(Equal(req.Subtotal))
			Expect(res.Shipping).To(Equal(req.Shipping))
			Expect(res.Tax).To(Equal(req.Tax))
			Expect(res.Total).To(Equal(req.Total))
			Expect(res.Company).To(Equal(req.Company))
			Expect(res.BillingAddress).To(Equal(req.BillingAddress))
			Expect(res.ShippingAddress).To(Equal(req.ShippingAddress))
			Expect(res.Gift).To(Equal(req.Gift))
			Expect(res.GiftMessage).To(Equal(req.GiftMessage))
			Expect(res.GiftEmail).To(Equal(req.GiftEmail))
		})
	})
})
