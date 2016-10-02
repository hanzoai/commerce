package test

import (
	"crowdstart.com/models/product"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("product", func() {
	Context("New product", func() {
		var req *product.Product
		var res *product.Product

		Before(func() {
			req = product.Fake(db)
			res = product.New(db)

			// Create new product
			cl.Post("/product", req, res)
		})

		It("Should create new products", func() {
			Expect(res.Name).To(Equal(req.Name))
			Expect(res.Headline).To(Equal(req.Headline))
			Expect(res.Description).To(Equal(req.Description))
			Expect(res.Slug).To(Equal(req.Slug))
			Expect(res.Currency).To(Equal(req.Currency))
			Expect(res.Price).To(Equal(req.Price))
			Expect(res.Shipping).To(Equal(req.Shipping))
			Expect(res.ListPrice).To(Equal(req.ListPrice))
		})
	})
})
