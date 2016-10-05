package test

import (
	"crowdstart.com/models/product"
	"crowdstart.com/models/variant"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("variant", func() {
	Context("New variant", func() {
		req := new(variant.Variant)
		res := new(variant.Variant)

		Before(func() {
			prod := product.Fake(db)
			prod.MustCreate()
			req = variant.Fake(db, prod.Id())
			res = variant.New(db)

			// Create new variant
			cl.Post("/variant", req, res)
		})

		It("Should create new variants", func() {
			Expect(res.ProductId).To(Equal(req.ProductId))
			Expect(res.SKU).To(Equal(req.SKU))
			Expect(res.Available).To(Equal(req.Available))
			Expect(res.Name).To(Equal(req.Name))
			Expect(res.Inventory).To(Equal(req.Inventory))
			Expect(res.Sold).To(Equal(req.Sold))
			Expect(res.Taxable).To(Equal(req.Taxable))
		})
	})
	Context("Get variant", func() {
		req := new(variant.Variant)
		res := new(variant.Variant)

		Before(func() {
			prod := product.Fake(db)
			prod.MustCreate()
			req = variant.Fake(db, prod.Id())
			req.MustCreate()

			res = variant.New(db)

			cl.Get("/variant/"+req.Id(), res)
		})

		It("Should get variants", func() {
			Expect(res.ProductId).To(Equal(req.ProductId))
			Expect(res.SKU).To(Equal(req.SKU))
			Expect(res.Available).To(Equal(req.Available))
			Expect(res.Name).To(Equal(req.Name))
			Expect(res.Inventory).To(Equal(req.Inventory))
			Expect(res.Sold).To(Equal(req.Sold))
			Expect(res.Taxable).To(Equal(req.Taxable))
		})
	})
	Context("Delete variant", func() {
		res := ""

		Before(func() {
			prod := product.Fake(db)
			prod.MustCreate()
			req := variant.Fake(db, prod.Id())
			req.MustCreate()

			cl.Delete("/variant/" + req.Id())
			res = req.Id()
		})

		It("Should delete variants", func() {
			vari := variant.New(db)
			err := vari.GetById(res)
			Expect(err).ToNot(BeNil())
		})
	})
})
