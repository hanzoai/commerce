package test

import (
	"crowdstart.com/models/discount"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("discount", func() {
	Context("New discount", func() {
		req := new(discount.Discount)
		res := new(discount.Discount)

		Before(func() {
			req = discount.Fake(db)
			res = discount.New(db)

			// Create new discount
			cl.Post("/discount", req, res)
		})

		It("Should create new discounts", func() {
			Expect(res.Type).To(Equal(req.Type))
			Expect(res.Name).To(Equal(req.Name))
			Expect(res.StartDate).To(Equal(req.StartDate))
			Expect(res.EndDate).To(Equal(req.EndDate))
			Expect(res.Scope.Type).To(Equal(req.Scope.Type))
			Expect(res.Target.Type).To(Equal(req.Target.Type))
		})
	})
	Context("Get discount", func() {
		req := new(discount.Discount)
		res := new(discount.Discount)

		Before(func() {
			req = discount.Fake(db)
			req.MustCreate()

			res = discount.New(db)

			// Get discount
			cl.Get("/discount/"+req.Id(), res)
		})

		It("Should get discounts", func() {
			Expect(res.Type).To(Equal(req.Type))
			Expect(res.Name).To(Equal(req.Name))
			Expect(res.StartDate.UTC()).To(Equal(req.StartDate.UTC()))
			Expect(res.EndDate.UTC()).To(Equal(req.EndDate.UTC()))
			Expect(res.Scope.Type).To(Equal(req.Scope.Type))
			Expect(res.Target.Type).To(Equal(req.Target.Type))
		})
	})
	Context("Delete discount", func() {
		res := ""

		Before(func() {
			req := discount.Fake(db)
			req.MustCreate()

			cl.Delete("/discount/" + req.Id())
			res = req.Id()
		})

		It("Should delete discounts", func() {
			d := discount.New(db)
			err := d.GetById(res)
			Expect(err).ToNot(BeNil())
		})
	})
})
