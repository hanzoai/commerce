package test

import (
	"strings"

	"crowdstart.com/models/coupon"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("coupon", func() {
	Context("New coupon", func() {
		var req *coupon.Coupon
		var res *coupon.Coupon

		Before(func() {
			req = coupon.Fake(db)
			res = coupon.New(db)

			// Create new coupon
			cl.Post("/coupon", req, res)
		})

		It("Should create new coupons", func() {
			Expect(res.Type).To(Equal(req.Type))
			Expect(res.Name).To(Equal(req.Name))
			Expect(res.Code_).To(Equal(strings.ToUpper(req.Code_)))
			Expect(res.Dynamic).To(Equal(req.Dynamic))
			Expect(res.StartDate).To(Equal(req.StartDate))
			Expect(res.EndDate).To(Equal(req.EndDate))
			Expect(res.Once).To(Equal(req.Once))
			Expect(res.Limit).To(Equal(req.Limit))
			Expect(res.Enabled).To(Equal(req.Enabled))
			Expect(res.Amount).To(Equal(req.Amount))
			Expect(res.Used).To(Equal(req.Used))
		})
	})
})
