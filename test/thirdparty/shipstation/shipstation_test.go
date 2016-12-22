package test

import (
	"crowdstart.com/models/lineitem"
	"crowdstart.com/models/order"
	"crowdstart.com/models/product"
	"crowdstart.com/models/variant"
	"crowdstart.com/util/log"
	"github.com/icrowley/fake"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("shipstation", func() {
	Context("Export", func() {
		Before(func() {
			prod := product.Fake(db)
			prod.MustCreate()
			vari := variant.Fake(db, prod.Id())
			vari.MustCreate()
			li := lineitem.Fake(vari)
			req := order.Fake(db, li)

			// Create orders
			cl.Post("/order", req, nil)
			cl.Post("/order", req, nil)
			cl.Post("/order", req, nil)
		})

		It("Should export orders", func() {
			w := cl.Get("/suchtees/shipstation?action=export", nil)
			log.Error(w.Body)
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

	Context("Patch product", func() {
		prod := new(product.Product)
		res := new(product.Product)

		req := struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}{
			fake.Word(),
			fake.Sentence(),
		}

		Before(func() {
			// Create product
			prod = product.Fake(db)
			prod.MustCreate()

			// Patch product
			cl.Patch("/product/"+prod.Id(), req, res)
		})

		It("Should patch product", func() {
			Expect(res.Id_).To(Equal(prod.Id()))
			Expect(res.Name).To(Equal(req.Name))
			Expect(res.Headline).To(Equal(prod.Headline))
			Expect(res.Description).To(Equal(req.Description))
			Expect(res.Slug).To(Equal(prod.Slug))
			Expect(res.Currency).To(Equal(prod.Currency))
			Expect(res.Price).To(Equal(prod.Price))
			Expect(res.Shipping).To(Equal(prod.Shipping))
			Expect(res.ListPrice).To(Equal(prod.ListPrice))
		})
	})

	Context("Put product", func() {
		prod := new(product.Product)
		res := new(product.Product)
		req := new(product.Product)

		Before(func() {
			prod = product.Fake(db)
			prod.MustCreate()

			// Create product request
			req = product.Fake(db)

			// Update product
			cl.Put("/product/"+prod.Id(), req, res)
		})

		It("Should put product", func() {
			Expect(res.Id_).To(Equal(prod.Id()))
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
