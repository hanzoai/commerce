package test

import (
	"crowdstart.com/models/product"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("product", func() {
	Context("New product", func() {
		req := new(product.Product)
		res := new(product.Product)

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
	Context("Get product", func() {
		req := new(product.Product)
		res := new(product.Product)

		Before(func() {
			req = product.Fake(db)
			req.MustCreate()
			res = product.New(db)

			cl.Get("/product/"+req.Id(), res)
		})

		It("Should get products", func() {
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
	Context("Delete product", func() {
		res := ""

		Before(func() {
			req := product.Fake(db)
			req.MustCreate()

			cl.Delete("/product/" + req.Id())
			res = req.Id()
		})

		It("Should get products", func() {
			prod := product.New(db)
			err := prod.GetById(res)
			Expect(err).ToNot(BeNil())
		})
	})
})
