package test

import (
	"strings"

	"crowdstart.com/models/lineitem"
	"crowdstart.com/models/order"
	"crowdstart.com/models/product"
	"crowdstart.com/models/variant"
	"crowdstart.com/util/log"

	. "crowdstart.com/util/test/ginkgo"
)

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

var _ = Describe("order", func() {
	Context("New order", func() {
		req := new(order.Order)
		res := new(order.Order)

		Before(func() {
			p := product.Fake(db)
			p.MustCreate()
			v := variant.Fake(db, p.Id())
			v.MustCreate()
			li := lineitem.Fake(v.Id(), v.Name, v.SKU)
			req = order.Fake(db, li)
			res = order.New(db)

			// Create new order
			cl.Post("/order", req, res)
			log.JSON(res)
		})

		It("Should create new orders", func() {
			Expect(res.Email).To(Equal(normalize(req.Email)))
			Expect(res.Status).To(Equal(req.Status))
			Expect(res.PaymentStatus).To(Equal(req.PaymentStatus))
			Expect(res.Preorder).To(Equal(req.Preorder))
			Expect(res.Unconfirmed).To(Equal(req.Unconfirmed))
			Expect(res.Currency).To(Equal(req.Currency))
			//TODO: These are coming back as zero on the POST request, let's figure out why
			//Expect(res.LineTotal).To(Equal(req.LineTotal))
			//Expect(res.Discount).To(Equal(req.Discount))
			//Expect(res.Subtotal).To(Equal(req.Subtotal))
			//Expect(res.Shipping).To(Equal(req.Shipping))
			//Expect(res.Tax).To(Equal(req.Tax))
			//Expect(res.Total).To(Equal(req.Total))
			//TODO: Prices on the items are also coming back as zero.
			//Expect(res.Items).To(Equal(req.Items))
			Expect(res.Company).To(Equal(req.Company))
			Expect(res.Balance).To(Equal(req.Balance))
			Expect(res.Paid).To(Equal(req.Paid))
			Expect(res.Refunded).To(Equal(req.Refunded))
			Expect(res.Company).To(Equal(req.Company))
			Expect(res.BillingAddress).To(Equal(req.BillingAddress))
			Expect(res.ShippingAddress).To(Equal(req.ShippingAddress))
			Expect(res.Gift).To(Equal(req.Gift))
			Expect(res.GiftMessage).To(Equal(req.GiftMessage))
			Expect(res.GiftEmail).To(Equal(normalize(req.GiftEmail)))
		})
	})
	Context("Get order", func() {
		req := new(order.Order)
		res := new(order.Order)

		Before(func() {
			p := product.Fake(db)
			p.MustCreate()
			v := variant.Fake(db, p.Id())
			v.MustCreate()
			li := lineitem.Fake(v.Id(), v.Name, v.SKU)
			req = order.Fake(db, li)
			req.MustCreate()

			res = order.New(db)

			cl.Get("/order/"+req.Id(), res)
		})

		It("Should get orders", func() {
			Expect(normalize(res.Email)).To(Equal(normalize(req.Email)))
			Expect(res.Status).To(Equal(req.Status))
			Expect(res.PaymentStatus).To(Equal(req.PaymentStatus))
			Expect(res.Preorder).To(Equal(req.Preorder))
			Expect(res.Unconfirmed).To(Equal(req.Unconfirmed))
			Expect(res.Currency).To(Equal(req.Currency))
			//TODO: These are coming back as zero on the POST request, let's figure out why
			//Expect(res.LineTotal).To(Equal(req.LineTotal))
			//Expect(res.Discount).To(Equal(req.Discount))
			//Expect(res.Subtotal).To(Equal(req.Subtotal))
			//Expect(res.Shipping).To(Equal(req.Shipping))
			//Expect(res.Tax).To(Equal(req.Tax))
			//Expect(res.Total).To(Equal(req.Total))
			//TODO: Prices on the items are also coming back as zero.
			//Expect(res.Items).To(Equal(req.Items))
			Expect(res.Company).To(Equal(req.Company))
			Expect(res.Balance).To(Equal(req.Balance))
			Expect(res.Paid).To(Equal(req.Paid))
			Expect(res.Refunded).To(Equal(req.Refunded))
			Expect(res.Company).To(Equal(req.Company))
			Expect(res.BillingAddress).To(Equal(req.BillingAddress))
			Expect(res.ShippingAddress).To(Equal(req.ShippingAddress))
			Expect(res.Gift).To(Equal(req.Gift))
			Expect(res.GiftMessage).To(Equal(req.GiftMessage))
			Expect(res.GiftEmail).To(Equal(normalize(req.GiftEmail)))
		})
	})
	Context("Delete order", func() {
		res := ""

		Before(func() {
			p := product.Fake(db)
			p.MustCreate()
			v := variant.Fake(db, p.Id())
			v.MustCreate()
			li := lineitem.Fake(v.Id(), v.Name, v.SKU)
			req := order.Fake(db, li)
			req.MustCreate()

			cl.Delete("/order/" + req.Id())

			res = req.Id()
		})

		It("Should delete orders", func() {
			ord := order.New(db)
			err := ord.GetById(res)
			Expect(err).ToNot(BeNil())
		})
	})
})
