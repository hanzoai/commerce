package test

import (
	"math/rand"
	"strings"
	"time"

	"github.com/icrowley/fake"

	"crowdstart.com/models/coupon"
	"crowdstart.com/util/log"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("coupon", func() {
	Context("New coupon", func() {
		req := new(coupon.Coupon)
		res := new(coupon.Coupon)

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

	Context("Get coupon", func() {
		req := new(coupon.Coupon)
		res := new(coupon.Coupon)

		Before(func() {
			// Create coupon
			req = coupon.Fake(db)
			req.MustCreate()

			// Make response for verification
			res = coupon.New(db)

			// Get coupon
			w := cl.Get("/coupon/"+req.Id(), res)
			log.Warn(w.Body.String())

		})

		It("Should get coupons", func() {
			Expect(res.Type).To(Equal(req.Type))
			Expect(res.Name).To(Equal(req.Name))
			Expect(res.Code_).To(Equal(strings.ToUpper(req.Code_)))
			Expect(res.Dynamic).To(Equal(req.Dynamic))
			Expect(res.StartDate.UTC()).To(Equal(req.StartDate))
			Expect(res.EndDate.UTC()).To(Equal(req.EndDate))
			Expect(res.Once).To(Equal(req.Once))
			Expect(res.Limit).To(Equal(req.Limit))
			Expect(res.Enabled).To(Equal(req.Enabled))
			Expect(res.Amount).To(Equal(req.Amount))
			Expect(res.Used).To(Equal(req.Used))
		})
	})

	Context("Put coupon", func() {
		coup := new(coupon.Coupon)
		res := new(coupon.Coupon)
		req := new(coupon.Coupon)

		Before(func() {
			coup = coupon.Fake(db)
			coup.MustCreate()

			// Create coupon request
			req = coupon.Fake(db)

			// Update coupon
			cl.Put("/coupon/"+coup.Id(), req, res)
		})

		It("Should put coupon", func() {
			Expect(res.Id_).To(Equal(coup.Id()))
			Expect(res.Type).To(Equal(req.Type))
			Expect(res.Name).To(Equal(req.Name))
			Expect(res.Code_).To(Equal(strings.ToUpper(req.Code_)))
			Expect(res.Dynamic).To(Equal(req.Dynamic))
			Expect(res.StartDate.UTC()).To(Equal(req.StartDate))
			Expect(res.EndDate.UTC()).To(Equal(req.EndDate))
			Expect(res.Once).To(Equal(req.Once))
			Expect(res.Limit).To(Equal(req.Limit))
			Expect(res.Enabled).To(Equal(req.Enabled))
			Expect(res.Amount).To(Equal(req.Amount))
			Expect(res.Used).To(Equal(req.Used))
		})
	})

	Context("patch coupon", func() {
		coup := new(coupon.Coupon)
		res := new(coupon.Coupon)

		req := struct {
			Name      string    `json:"name"`
			StartDate time.Time `json:"startDate"`
			EndDate   time.Time `json:"endDate"`
		}{
			fake.FullName(),
			time.Date(rand.Intn(25)+2000, time.Month(rand.Intn(12)+1), rand.Intn(25)+1, 0, 0, 0, 0, time.UTC).UTC(),
			time.Date(rand.Intn(25)+2025, time.Month(rand.Intn(12)+1), rand.Intn(25)+1, 0, 0, 0, 0, time.UTC).UTC(),
		}

		Before(func() {
			coup = coupon.Fake(db)
			coup.MustCreate()

			// Update coupon
			cl.Put("/coupon/"+coup.Id(), req, res)
			log.JSON(req)
			log.JSON(res)
		})

		It("Should patch coupon", func() {
			Expect(res.Id_).To(Equal(coup.Id()))
			Expect(res.Type).To(Equal(coup.Type))
			Expect(res.Name).To(Equal(req.Name))
			Expect(res.Code_).To(Equal(strings.ToUpper(coup.Code_)))
			Expect(res.Dynamic).To(Equal(coup.Dynamic))
			Expect(res.StartDate.UTC()).To(Equal(req.StartDate.UTC()))
			Expect(res.EndDate.UTC()).To(Equal(req.EndDate.UTC()))
			Expect(res.Once).To(Equal(coup.Once))
			Expect(res.Limit).To(Equal(coup.Limit))
			Expect(res.Enabled).To(Equal(coup.Enabled))
			Expect(res.Amount).To(Equal(coup.Amount))
			Expect(res.Used).To(Equal(coup.Used))
		})
	})

	Context("Delete coupon", func() {
		res := ""

		Before(func() {
			// Create coupon
			req := coupon.Fake(db)
			req.MustCreate()

			// Delete it
			cl.Delete("/coupon/" + req.Id())

			res = req.Id()
		})

		It("Should delete coupons", func() {
			cpn := coupon.New(db)
			err := cpn.GetById(res)
			Expect(err).ToNot(BeNil())
		})
	})
})
