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
		req := new(referrer.Referrer)
		res := new(referrer.Referrer)

		Before(func() {
			prod := product.Fake(db)
			prod.MustCreate()
			vari := variant.Fake(db, prod.Id())
			vari.MustCreate()
			li := lineitem.Fake(vari.Id(), vari.Name, vari.SKU)
			ord := order.Fake(db, li)
			ord.MustCreate()
			usr := user.Fake(db)
			usr.MustCreate()
			req = referrer.Fake(db, usr.Id(), ord.Id())
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
	Context("Get referrer", func() {
		req := new(referrer.Referrer)
		res := new(referrer.Referrer)

		Before(func() {
			prod := product.Fake(db)
			prod.MustCreate()
			vari := variant.Fake(db, prod.Id())
			vari.MustCreate()
			li := lineitem.Fake(vari.Id(), vari.Name, vari.SKU)
			ord := order.Fake(db, li)
			ord.MustCreate()
			usr := user.Fake(db)
			usr.MustCreate()
			req = referrer.Fake(db, usr.Id(), ord.Id())
			req.MustCreate()

			res = referrer.New(db)

			cl.Get("/referrer/"+req.Id(), res)
		})

		It("Should get referrers", func() {
			Expect(res.Code).To(Equal(req.Code))
			Expect(res.OrderId).To(Equal(req.OrderId))
			Expect(res.UserId).To(Equal(req.UserId))
			Expect(res.FirstReferredAt.UTC()).To(Equal(req.FirstReferredAt.UTC()))
		})
	})
	Context("Delete referrer", func() {
		res := ""

		Before(func() {
			p := product.Fake(db)
			p.MustCreate()
			v := variant.Fake(db, p.Id())
			v.MustCreate()
			li := lineitem.Fake(v.Id(), v.Name, v.SKU)
			ord := order.Fake(db, li)
			ord.MustCreate()
			usr := user.Fake(db)
			usr.MustCreate()
			req := referrer.Fake(db, usr.Id(), ord.Id())
			req.MustCreate()

			cl.Delete("/referrer/" + req.Id())
			res = req.Id()
		})

		It("Should get referrers", func() {
			refer := referrer.New(db)
			err := refer.GetById(res)
			Expect(err).ToNot(BeNil())
		})
	})
})
