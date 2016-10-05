package test

import (
	"crowdstart.com/models/lineitem"
	"crowdstart.com/models/order"
	"crowdstart.com/models/product"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/user"
	"crowdstart.com/models/variant"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("referral", func() {
	Context("New referral", func() {
		req := new(referral.Referral)
		res := new(referral.Referral)

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
			req = referral.Fake(db, usr.Id(), ord.Id())
			res = referral.New(db)

			// Create new referral
			cl.Post("/referral", req, res)
		})

		It("Should create new referrals", func() {
			Expect(res.Type).To(Equal(req.Type))
			Expect(res.OrderId).To(Equal(req.OrderId))
			Expect(res.Referrer).To(Equal(req.Referrer))
			Expect(res.Fee).To(Equal(req.Fee))
		})
	})
	Context("Get referral", func() {
		req := new(referral.Referral)
		res := new(referral.Referral)

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
			req = referral.Fake(db, usr.Id(), ord.Id())
			req.MustCreate()

			res = referral.New(db)

			cl.Get("/referral/"+req.Id(), res)
		})

		It("Should get referrals", func() {
			Expect(res.Type).To(Equal(req.Type))
			Expect(res.OrderId).To(Equal(req.OrderId))
			Expect(res.Referrer).To(Equal(req.Referrer))
			Expect(res.Fee).To(Equal(req.Fee))
		})
	})
	Context("Delete referral", func() {
		res := ""

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
			req := referral.Fake(db, usr.Id(), ord.Id())
			req.MustCreate()

			// Create new referral
			cl.Delete("/referral/" + req.Id())
			res = req.Id()
		})

		It("Should delete referrals", func() {
			ref := referral.New(db)
			err := ref.GetById(res)
			Expect(err).ToNot(BeNil())
		})
	})
})
