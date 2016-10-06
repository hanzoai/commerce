package test

import (
	"crowdstart.com/models/cart"
	"crowdstart.com/models/user"
	"crowdstart.com/util/log"
	"github.com/icrowley/fake"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("cart", func() {
	Context("New cart", func() {
		req := new(cart.Cart)
		res := new(cart.Cart)

		Before(func() {
			usr := user.Fake(db)
			usr.MustCreate()
			req = cart.Fake(db, usr.Id())
			res = cart.New(db)

			// Create new cart
			cl.Post("/cart", req, res)
		})

		It("Should create new carts", func() {
			Expect(res.UserId).To(Equal(req.UserId))
			Expect(res.Email).To(Equal(req.Email))
			Expect(res.ReferrerId).To(Equal(req.ReferrerId))
			Expect(res.Status).To(Equal(req.Status))
			Expect(res.Currency).To(Equal(req.Currency))
			Expect(res.LineTotal).To(Equal(req.LineTotal))
			Expect(res.Discount).To(Equal(req.Discount))
			Expect(res.Subtotal).To(Equal(req.Subtotal))
			Expect(res.Shipping).To(Equal(req.Shipping))
			Expect(res.Tax).To(Equal(req.Tax))
			Expect(res.Total).To(Equal(req.Total))
			Expect(res.Company).To(Equal(req.Company))
			Expect(res.BillingAddress).To(Equal(req.BillingAddress))
			Expect(res.ShippingAddress).To(Equal(req.ShippingAddress))
			Expect(res.Gift).To(Equal(req.Gift))
			Expect(res.GiftMessage).To(Equal(req.GiftMessage))
			Expect(res.GiftEmail).To(Equal(req.GiftEmail))
		})
	})
	Context("Get cart", func() {
		req := new(cart.Cart)
		res := new(cart.Cart)

		Before(func() {
			// Create user and cart
			usr := user.Fake(db)
			usr.MustCreate()

			req = cart.Fake(db, usr.Id())
			req.MustCreate()

			// Verify it exists
			res = cart.New(db)

			// Get cart
			cl.Get("/cart/"+req.Id(), res)
		})

		It("Should create new carts", func() {
			Expect(res.UserId).To(Equal(req.UserId))
			Expect(res.Email).To(Equal(req.Email))
			Expect(res.ReferrerId).To(Equal(req.ReferrerId))
			Expect(res.Status).To(Equal(req.Status))
			Expect(res.Currency).To(Equal(req.Currency))
			Expect(res.LineTotal).To(Equal(req.LineTotal))
			Expect(res.Discount).To(Equal(req.Discount))
			Expect(res.Subtotal).To(Equal(req.Subtotal))
			Expect(res.Shipping).To(Equal(req.Shipping))
			Expect(res.Tax).To(Equal(req.Tax))
			Expect(res.Total).To(Equal(req.Total))
			Expect(res.Company).To(Equal(req.Company))
			Expect(res.BillingAddress).To(Equal(req.BillingAddress))
			Expect(res.ShippingAddress).To(Equal(req.ShippingAddress))
			Expect(res.Gift).To(Equal(req.Gift))
			Expect(res.GiftMessage).To(Equal(req.GiftMessage))
			Expect(res.GiftEmail).To(Equal(req.GiftEmail))
		})
	})
	Context("Delete cart", func() {
		res := ""

		Before(func() {
			usr := user.Fake(db)
			usr.MustCreate()

			req := cart.Fake(db, usr.Id())
			req.MustCreate()

			cl.Delete("/cart/" + req.Id())

			res = req.Id()
		})

		It("Should delete carts", func() {
			car := cart.New(db)
			err := car.GetById(res)
			Expect(err).ToNot(BeNil())
		})
	})

	Context("Put cart", func() {
		car := new(cart.Cart)
		req := new(cart.Cart)
		res := new(cart.Cart)

		Before(func() {
			// Create user
			usr := user.Fake(db)
			usr.MustCreate()

			// Create cart
			car := cart.Fake(db, usr.Id())
			car.MustCreate()

			// Create new cart for update
			req = cart.Fake(db, usr.Id())

			// Update cart
			cl.Put("/cart/"+car.Id(), req, res)
		})

		It("Should put cart", func() {
			Expect(res.Id_).To(Equal(car.Id()))
			Expect(res.UserId).To(Equal(req.UserId))
			Expect(res.Email).To(Equal(req.Email))
			Expect(res.ReferrerId).To(Equal(req.ReferrerId))
			Expect(res.Status).To(Equal(req.Status))
			Expect(res.Currency).To(Equal(req.Currency))
			Expect(res.LineTotal).To(Equal(req.LineTotal))
			Expect(res.Discount).To(Equal(req.Discount))
			Expect(res.Subtotal).To(Equal(req.Subtotal))
			Expect(res.Shipping).To(Equal(req.Shipping))
			Expect(res.Tax).To(Equal(req.Tax))
			Expect(res.Total).To(Equal(req.Total))
			Expect(res.Company).To(Equal(req.Company))
			Expect(res.BillingAddress).To(Equal(req.BillingAddress))
			Expect(res.ShippingAddress).To(Equal(req.ShippingAddress))
			Expect(res.Gift).To(Equal(req.Gift))
			Expect(res.GiftMessage).To(Equal(req.GiftMessage))
			Expect(res.GiftEmail).To(Equal(req.GiftEmail))
		})
	})

	FContext("Patch cart", func() {
		car := new(cart.Cart)
		res := new(cart.Cart)

		req := struct {
			Email   string `json:"email"`
			Company string `json:"company"`
		}{
			fake.EmailAddress(),
			fake.Company(),
		}

		Before(func() {
			// Create user and cart
			usr := user.Fake(db)
			usr.MustCreate()

			car = cart.Fake(db, usr.Id())
			car.MustCreate()

			res = cart.New(db)

			// patch cart
			cl.Patch("/cart/"+car.Id(), req, res)
			log.JSON(req)
			log.JSON(res)
		})

		It("Should patch cart", func() {
			Expect(res.Id_).To(Equal(car.Id()))
			Expect(res.Email).To(Equal(req.Email))
			Expect(res.Company).To(Equal(req.Company))
			Expect(res.UserId).To(Equal(car.UserId))
			Expect(res.ReferrerId).To(Equal(car.ReferrerId))
			Expect(res.Status).To(Equal(car.Status))
			Expect(res.Currency).To(Equal(car.Currency))
			Expect(res.LineTotal).To(Equal(car.LineTotal))
			Expect(res.Discount).To(Equal(car.Discount))
			Expect(res.Subtotal).To(Equal(car.Subtotal))
			Expect(res.Shipping).To(Equal(car.Shipping))
			Expect(res.Tax).To(Equal(car.Tax))
			Expect(res.Total).To(Equal(car.Total))
			Expect(res.BillingAddress).To(Equal(car.BillingAddress))
			Expect(res.ShippingAddress).To(Equal(car.ShippingAddress))
			Expect(res.Gift).To(Equal(car.Gift))
			Expect(res.GiftMessage).To(Equal(car.GiftMessage))
			Expect(res.GiftEmail).To(Equal(car.GiftEmail))
		})
	})
})
