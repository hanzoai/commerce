package test

import (
	"crowdstart.com/models/lineitem"
	"crowdstart.com/models/order"
	"crowdstart.com/models/product"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/user"
	"crowdstart.com/models/variant"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("referrer", func() {
	Context("New referrer", func() {
		var req *referrer.Referrer
		var res *referrer.Referrer

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
			req = referrer.Fake(db, usr.Id(), o.Id())
			res = referrer.New(db)

			// Create new referrer
			cl.Post("/referrer", req, res)
		})

		It("Should create new referrers", func() {
			Expect(res.Code).To(Equal(req.Code))
			Expect(res.OrderId).To(Equal(req.OrderId))
			Expect(res.UserId).To(Equal(req.UserId))
			Expect(res.FirstReferredAt).To(Equal(req.FirstReferredAt))
		})
	})
})
