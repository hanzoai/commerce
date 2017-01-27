package test

import (
	"crowdstart.com/models/product"
	"crowdstart.com/models/review"
	"crowdstart.com/models/user"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("review", func() {
	Context("New review", func() {
		req := new(review.Review)
		res := new(review.Review)

		Before(func() {
			usr := user.Fake(db)
			usr.MustCreate()

			prod := product.Fake(db)
			prod.MustCreate()

			req = review.Fake(db, usr.Id(), prod.Id())
			res = review.New(db)

			// Create new referrer
			cl.Post("/review", req, res)
		})

		It("Should create new reviews", func() {
			Expect(res.UserId).To(Equal(req.UserId))
			Expect(res.ProductId).To(Equal(req.ProductId))
			Expect(res.Name).To(Equal(req.Name))
			Expect(res.Comment).To(Equal(req.Comment))
			Expect(res.Rating).To(Equal(req.Rating))
		})
	})

	Context("Get review", func() {
		req := new(review.Review)
		res := new(review.Review)

		Before(func() {
			usr := user.Fake(db)
			usr.MustCreate()

			prod := product.Fake(db)
			prod.MustCreate()

			req = review.Fake(db, usr.Id(), prod.Id())
			req.MustCreate()

			res = review.New(db)
			cl.Get("/review/"+req.Id(), res)
		})

		It("Should create new reviews", func() {
			Expect(res.UserId).To(Equal(req.UserId))
			Expect(res.ProductId).To(Equal(req.ProductId))
			Expect(res.Name).To(Equal(req.Name))
			Expect(res.Comment).To(Equal(req.Comment))
			Expect(res.Rating).To(Equal(req.Rating))
		})
	})
})
