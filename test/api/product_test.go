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
		})
	})
})
