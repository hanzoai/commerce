package test

import (
	"crowdstart.com/models/lineitem"
	"crowdstart.com/models/order"
	"crowdstart.com/models/product"
	"crowdstart.com/models/variant"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("order", func() {
	Context("New order", func() {
		var req *order.Order
		var res *order.Order

		Before(func() {
			p := product.Fake(db)
			v := variant.Fake(db, p.Id())
			li := lineitem.Fake(v.Id(), v.Name, v.SKU)
			req = order.Fake(db, li)
			res = order.New(db)

			// Create new order
			cl.Post("/order", req, res)
		})

		It("Should create new orders", func() {
			Expect(res.Email).To(Equal(req.Email))
			Expect(res.Status).To(Equal(req.Status))
			Expect(res.PaymentStatus).To(Equal(req.PaymentStatus))
			Expect(res.Preorder).To(Equal(req.Preorder))
			Expect(res.Unconfirmed).To(Equal(req.Unconfirmed))
			Expect(res.Currency).To(Equal(req.Currency))
			Expect(res.LineTotal).To(Equal(req.LineTotal))
			Expect(res.Discount).To(Equal(req.Discount))
			Expect(res.Subtotal).To(Equal(req.Subtotal))
			Expect(res.Shipping).To(Equal(req.Shipping))
			Expect(res.Tax).To(Equal(req.Tax))
			Expect(res.Total).To(Equal(req.Total))
			Expect(res.Company).To(Equal(req.Company))
			Expect(res.Balance).To(Equal(req.Balance))
			Expect(res.Paid).To(Equal(req.Paid))
			Expect(res.Refunded).To(Equal(req.Refunded))
			Expect(res.Company).To(Equal(req.Company))
			Expect(res.BillingAddress).To(Equal(req.BillingAddress))
			Expect(res.ShippingAddress).To(Equal(req.ShippingAddress))
			Expect(res.Gift).To(Equal(req.Gift))
			Expect(res.GiftMessage).To(Equal(req.GiftMessage))
			Expect(res.GiftEmail).To(Equal(req.GiftEmail))
			Expect(res.Items).To(Equal(req.Items))
		})
	})
})
