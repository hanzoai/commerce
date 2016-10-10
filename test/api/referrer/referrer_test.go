package test

import (
	"math/rand"
	"time"

	"crowdstart.com/models/lineitem"
	"crowdstart.com/models/order"
	"crowdstart.com/models/product"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/user"
	"crowdstart.com/models/variant"
	"github.com/icrowley/fake"

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
			li := lineitem.Fake(vari)
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
			li := lineitem.Fake(vari)
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

	Context("Patch referrer", func() {
		re := new(referrer.Referrer)
		res := new(referrer.Referrer)

		req := struct {
			Code            string    `json:"Code"`
			FirstReferredAt time.Time `json:"firstReferredAt"`
		}{
			fake.Word(),
			time.Date(rand.Intn(15)+2000, time.Month(rand.Intn(12)+1), rand.Intn(25)+1, 0, 0, 0, 0, time.UTC),
		}

		Before(func() {
			prod := product.Fake(db)
			prod.MustCreate()
			vari := variant.Fake(db, prod.Id())
			vari.MustCreate()
			li := lineitem.Fake(vari)
			ord := order.Fake(db, li)
			ord.MustCreate()
			usr := user.Fake(db)
			usr.MustCreate()
			re = referrer.Fake(db, usr.Id(), ord.Id())
			re.MustCreate()

			// Patch referrer
			cl.Patch("/referrer/"+re.Id(), req, res)
		})

		It("Should patch referrer", func() {
			Expect(res.Id_).To(Equal(re.Id()))
			Expect(res.Code).To(Equal(req.Code))
			Expect(res.OrderId).To(Equal(re.OrderId))
			Expect(res.UserId).To(Equal(re.UserId))
			Expect(res.FirstReferredAt.UTC()).To(Equal(req.FirstReferredAt.UTC()))
		})
	})

	Context("Put referrer", func() {
		re := new(referrer.Referrer)
		res := new(referrer.Referrer)
		req := new(referrer.Referrer)

		Before(func() {
			prod := product.Fake(db)
			prod.MustCreate()
			vari := variant.Fake(db, prod.Id())
			vari.MustCreate()
			li := lineitem.Fake(vari)
			ord := order.Fake(db, li)
			ord.MustCreate()
			usr := user.Fake(db)
			usr.MustCreate()
			re = referrer.Fake(db, usr.Id(), ord.Id())
			re.MustCreate()

			req = referrer.Fake(db, usr.Id(), ord.Id())

			// Update referrer
			cl.Put("/referrer/"+re.Id(), req, res)
		})

		It("Should put referrer", func() {
			Expect(res.Id_).To(Equal(re.Id()))
			Expect(res.Code).To(Equal(req.Code))
			Expect(res.OrderId).To(Equal(req.OrderId))
			Expect(res.UserId).To(Equal(req.UserId))
			Expect(res.FirstReferredAt.UTC()).To(Equal(req.FirstReferredAt.UTC()))
		})
	})

	Context("Delete referrer", func() {
		res := ""

		Before(func() {
			prod := product.Fake(db)
			prod.MustCreate()
			vari := variant.Fake(db, prod.Id())
			vari.MustCreate()
			li := lineitem.Fake(vari)
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
