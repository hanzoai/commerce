package test

import (
	"crowdstart.com/models/discount"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("discount", func() {
	Context("New discount", func() {
		var req *discount.Discount
		var res *discount.Discount

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
})
