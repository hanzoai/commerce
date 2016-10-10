package test

import (
	"crowdstart.com/api/checkout"
	"crowdstart.com/models/affiliate"
	"crowdstart.com/models/lineitem"
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/product"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/user"
	"crowdstart.com/models/variant"
	"crowdstart.com/util/hashid"

	. "crowdstart.com/util/test/ginkgo"
)

func keyExists(key string) {
	ok, err := hashid.KeyExists(db.Context, key)
	Expect1(err).ToNot(HaveOccurred())
	Expect1(ok).To(BeTrue())
}

func getOrder(id string) *order.Order {
	ord := order.New(db)
	err := ord.GetById(id)
	Expect1(err).ToNot(HaveOccurred())
	return ord
}

func getUser(id string) *user.User {
	usr := user.New(db)
	err := usr.GetById(id)
	Expect1(err).ToNot(HaveOccurred())
	return usr
}

func getPayment(id string) *payment.Payment {
	pay := payment.New(db)
	err := pay.GetById(id)
	Expect1(err).ToNot(HaveOccurred())
	return pay
}

var _ = Describe("/checkout/authorize", func() {
	Context("Authorize new user", func() {
		var req *checkout.Authorization
		var res *order.Order

		Before(func() {
			// Create fake product, variant and order
			prod := product.Fake(db)
			prod.MustCreate()
			vari := variant.Fake(db, prod.Id())
			vari.MustCreate()
			li := lineitem.Fake(vari)
			ord := order.Fake(db, li)

			// Create new authorization request
			req = new(checkout.Authorization)
			req.Order = ord
			req.Payment = payment.Fake(db)
			req.User = user.Fake(db)

			// Instantiate order to encompass result
			res = order.New(db)

			// Make request
			cl.Post("/checkout/authorize", req, res)
		})

		It("Should save user", func() {
			getUser(res.UserId)
		})

		It("Should save payment", func() {
			getPayment(res.PaymentIds[0])
		})

		It("Should save order", func() {
			getOrder(res.Id())
		})

		It("Should save payment id on order", func() {
			Expect(len(res.PaymentIds)).To(Equal(1))
		})

		It("Should calculate correct total for order and payment", func() {
			Expect(res.Total).To(Equal(req.Order.Total))
		})
	})

	Context("Authorize invalid product", func() {
		var req *checkout.Authorization

		Before(func() {
			// Create fake product, variant and order
			prod := product.Fake(db)
			prod.MustCreate()
			vari := variant.Fake(db, prod.Id())
			vari.MustCreate()
			li := lineitem.Fake(vari)
			ord := order.Fake(db, li)

			// Create new authorization request
			req = new(checkout.Authorization)
			req.Order = ord
			req.Payment = payment.Fake(db)
			req.User = user.Fake(db)
		})

		It("Should not authorize invalid product id", func() {

		})
	})

	Context("Authorize invalid variant", func() {
		It("Should not authorize invalid variant id", func() {

		})
	})

	Context("Authorize invalid collection", func() {
		It("Should not authorize invalid collection id", func() {

		})
	})

	Context("Authorize existing user", func() {
		var req *checkout.Authorization
		var res *order.Order
		var usr *user.User

		Before(func() {
			// Create returning user
			usr = user.Fake(db)
			usr.MustCreate()

			// Create fake product, variant and order
			prod := product.Fake(db)
			prod.MustCreate()
			vari := variant.Fake(db, prod.Id())
			vari.MustCreate()
			li := lineitem.Fake(vari)
			ord := order.Fake(db, li)

			// Create new authorization request
			req = new(checkout.Authorization)
			req.Order = ord
			req.Payment = payment.Fake(db)
			req.User = usr

			// Instantiate order to encompass result
			res = order.New(db)

			// Make request
			cl.Post("/checkout/authorize", req, res)
		})

		It("Should re-use user successfully", func() {
			Expect(res.UserId).To(Equal(usr.Id()))
		})
	})

	Context("Authorize invalid user", func() {
		var req *checkout.Authorization

		Before(func() {
			// Create invalid user
			usr := user.Fake(db)
			usr.Id() // Allocate id, but don't create

			// Create fake product, variant and order
			prod := product.Fake(db)
			prod.MustCreate()
			vari := variant.Fake(db, prod.Id())
			vari.MustCreate()
			li := lineitem.Fake(vari)
			ord := order.Fake(db, li)

			// Create new authorization request
			req = new(checkout.Authorization)
			req.Order = ord
			req.Payment = payment.Fake(db)
			req.User = usr
		})

		It("Should not allow authorization with invalid user id", func() {
			// Make request
			cl.Post("/checkout/authorize", req, nil, 400)
		})
	})

	Context("Authorize with affiliate", func() {
		var req *checkout.Authorization

		Before(func() {
			// Create affiliate user
			usr := user.Fake(db)
			usr.MustCreate()

			// Create affiliate
			aff := affiliate.Fake(db, usr.Id())
			aff.MustCreate()

			// Create referrer for order request
			ref := referrer.Fake(db, usr.Id())
			ref.AffiliateId = aff.Id()
			ref.MustCreate()

			// Create order user
			usr = user.Fake(db)

			// Create fake product, variant and order
			prod := product.Fake(db)
			prod.MustCreate()
			vari := variant.Fake(db, prod.Id())
			vari.MustCreate()
			li := lineitem.Fake(vari)
			ord := order.Fake(db, li)

			// Create new authorization request
			req = new(checkout.Authorization)
			req.Order = ord
			req.Payment = payment.Fake(db)
			req.User = usr
		})
	})

	Context("Charge First Time Customers", func() {
		It("Should save new order successfully", func() {
		})

		It("Should save new order successfully for store", func() {
		})

		It("Should not authorize invalid credit card number", func() {
		})

		It("Should not authorize invalid credit card number for store", func() {
		})

		It("Should not authorize invalid product id", func() {
		})

		It("Should not authorize invalid variant id", func() {
		})

		It("Should not authorize invalid collection id", func() {
		})
	})

	Context("Charge Returning Customers", func() {
		It("Should save returning customer order with the same card successfully", func() {
		})

		It("Should save returning customer order with the same card successfully for store", func() {
		})

		It("Should save returning customer order with a new card successfully", func() {
		})

		It("Should save returning customer order with a new card successfully for store", func() {
		})

		It("Should not save customer with invalid user id", func() {
		})

		It("Should not save customer with invalid user id", func() {
		})
	})

	Context("Authorize Order", func() {
		It("Should authorize existing order successfully", func() {
		})

		It("Should not authorize invalid order", func() {
		})

		It("Should authorize order with coupon successfully", func() {
		})
	})

	Context("Capture Order", func() {
		It("Should capture existing authorized order successfully", func() {
		})

		It("Should not capture invalid order", func() {
		})
	})

	Context("Charge Order", func() {
		It("Should charge existing order successfully", func() {
		})

		It("Should not capture invalid order", func() {
		})
	})

	Context("Charge Order With Referral", func() {
		It("Should charge existing order with referral successfully", func() {
		})
	})

	Context("Charge Order With Single Use Coupon", func() {
		It("Should charge order with single use coupon successfully", func() {
		})
	})

	Context("Charge Order With Discount Rules Applicable", func() {
		It("Should charge order and apply appropriate discount rules", func() {
		})
	})

	Context("Refund Order", func() {
		It("Should refund order successfully", func() {
		})
	})
})
