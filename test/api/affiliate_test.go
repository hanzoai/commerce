package test

import (
	"crowdstart.com/models/affiliate"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("affiliate", func() {
	Context("New affiliate", func() {
		var req *affiliate.Affiliate
		var res *affiliate.Affiliate

		Before(func() {
			req = affiliate.Fake(db)
			res = affiliate.New(db)

			// Create new affiliate
			cl.Post("/affiliate", req, res)
		})

		It("Should create new affiliates", func() {
			Expect(res.Name).To(Equal(req.Name))
			Expect(res.Company).To(Equal(req.TaxId))
			Expect(res.Country).To(Equal(req.TaxId))
			Expect(res.TaxId).To(Equal(req.TaxId))
			Expect(res.Commission.Flat).To(Equal(req.Commission.Flat))
			Expect(res.Commission.Minimum).To(Equal(req.Commission.Minimum))
			Expect(res.Commission.Percent).To(Equal(req.Commission.Percent))
		})
	})
})
