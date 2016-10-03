package test

import (
	"crowdstart.com/models/lineitem"
	"crowdstart.com/models/order"
	"crowdstart.com/models/product"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/user"
	"crowdstart.com/models/variant"
	"crowdstart.com/util/log"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("referral", func() {
	Context("New referral", func() {
		var req *referral.Referral
		var res *referral.Referral

		Before(func() {
			p := product.Fake(db)
			p.MustCreate()
			v := variant.Fake(db, p.Id())
			v.MustCreate()
			li := lineitem.Fake(v.Id(), v.Name, v.SKU)
			o := order.Fake(db, li)
			o.MustCreate()
			usr := user.Fake(db)
			usr.MustCreate()
			req = referral.Fake(db, usr.Id(), o.Id())
			res = referral.New(db)

			// Create new referral
			cl.Post("/referral", req, res)
			log.JSON(res)
		})

		It("Should create new referrals", func() {
			Expect(res.Type).To(Equal(req.Type))
			Expect(res.OrderId).To(Equal(req.OrderId))
			Expect(res.Referrer).To(Equal(req.Referrer))
			Expect(res.Fee).To(Equal(req.Fee))
		})
	})
})
