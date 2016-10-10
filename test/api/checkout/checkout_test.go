package test

import (
	"math"
	"time"

	"crowdstart.com/api/checkout"
	"crowdstart.com/models/affiliate"
	"crowdstart.com/models/fee"
	"crowdstart.com/models/lineitem"
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/product"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/types/pricing"
	"crowdstart.com/models/user"
	"crowdstart.com/models/variant"
	"crowdstart.com/util/hashid"
	"crowdstart.com/util/log"

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

func getPayment(orderId string) *payment.Payment {
	pay := payment.New(db)
	ok, err := pay.Query().Filter("OrderId=", orderId).First()
	Expect1(err).ToNot(HaveOccurred())
	Expect1(ok).To(BeTrue())
	return pay
}

func getFee(paymentId, feeType string) *fee.Fee {
	fe := fee.New(db)
	ok, err := fe.Query().Filter("PaymentId=", paymentId).Filter("Type=", feeType).First()
	Expect1(err).ToNot(HaveOccurred())
	Expect1(ok).To(BeTrue())
	return fe
}

func getReferral(orderId string) *referral.Referral {
	rfl := referral.New(db)
	ok, err := rfl.Query().Filter("OrderId=", orderId).First()
	Expect1(err).ToNot(HaveOccurred())
	Expect1(ok).To(BeTrue())
	return rfl
}

func calculatePlatformFee(pricing pricing.Fees, total currency.Cents) currency.Cents {
	pctFee := math.Ceil(float64(total) * pricing.Card.Percent)
	return pricing.Card.Flat + currency.Cents(pctFee)
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
			getPayment(res.Id())
		})

		It("Should save order", func() {
			getOrder(res.Id())
		})

		It("Should save payment id on order", func() {
			Expect(len(res.PaymentIds)).To(Equal(1))
		})

		It("Should calculate correct total for order and payment", func() {
			log.JSON("REQUEST", req.Order)
			log.JSON("RESPONSE", res)
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

	Context("Authorize with referrer", func() {
		var req *checkout.Authorization
		var res *order.Order
		var ref *referrer.Referrer

		Before(func() {
			// Create affiliate user
			usr := user.Fake(db)
			usr.MustCreate()

			// Create referrer for order request
			ref := referrer.Fake(db, usr.Id())
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
			ord.ReferrerId = ref.Id()

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

		It("Should save referrer information", func() {
			Expect(res.ReferrerId).To(Equal(ref.Id()))
		})

		It("Should save referral", func() {
			getReferral(res.Id())
		})

		It("Should save platform fee", func() {
			pay := getPayment(res.Id())
			fe := getFee(pay.Id(), "platform")
			Expect(fe.Amount).To(Equal(calculatePlatformFee(org.Fees, res.Total)))
		})
	})

	Context("Charge with affiliate", func() {
		var req *checkout.Authorization
		var res *order.Order
		var aff *affiliate.Affiliate

		Before(func() {
			// Create affiliate user
			usr := user.Fake(db)
			usr.MustCreate()

			// Create affiliate
			aff = affiliate.Fake(db, usr.Id())
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
			ord.ReferrerId = ref.Id()

			// Create new authorization request
			req = new(checkout.Authorization)
			req.Order = ord
			req.Payment = payment.Fake(db)
			req.User = usr

			// Instantiate order to encompass result
			res = order.New(db)

			// Make request
			cl.Post("/checkout/charge", req, res)
			time.Sleep(time.Second * 5)
		})

		It("Should save referral for affiliate", func() {
			rfl := getReferral(res.Id())
			Expect(rfl.Referrer.AffiliateId).To(Equal(aff.Id()))
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
