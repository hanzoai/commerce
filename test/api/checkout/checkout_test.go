package test

import (
	"math"

	"crowdstart.com/api/checkout"
	"crowdstart.com/datastore"
	"crowdstart.com/models/affiliate"
	"crowdstart.com/models/fee"
	"crowdstart.com/models/lineitem"
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/product"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/types/commission"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/types/pricing"
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

func getPayment(orderId string) *payment.Payment {
	pay := payment.New(db)
	ok, err := pay.Query().Filter("OrderId=", orderId).First()
	Expect1(err).ToNot(HaveOccurred())
	Expect1(ok).To(BeTrue())
	return pay
}

func getPaymentByParent(key datastore.Key) *payment.Payment {
	pay := payment.New(db)
	ok, err := pay.Query().Ancestor(key).First()
	Expect1(err).ToNot(HaveOccurred())
	Expect1(ok).To(BeTrue())
	return pay
}

func getOrderByParent(key datastore.Key) *order.Order {
	ord := order.New(db)
	ok, err := ord.Query().Ancestor(key).First()
	Expect1(err).ToNot(HaveOccurred())
	Expect1(ok).To(BeTrue())
	return ord
}

func getFees(paymentId, feeType string) []*fee.Fee {
	fees := make([]*fee.Fee, 0)
	_, err := fee.Query(db).
		Filter("PaymentId=", paymentId).
		Filter("Type=", feeType).
		GetAll(&fees)
	Expect1(err).ToNot(HaveOccurred())
	return fees
}

func getReferral(orderId string) *referral.Referral {
	rfl := referral.New(db)
	ok, err := rfl.Query().Filter("OrderId=", orderId).First()
	Expect1(err).ToNot(HaveOccurred())
	Expect1(ok).To(BeTrue())
	return rfl
}

func calcPlatformFee(pricing pricing.Fees, total currency.Cents) currency.Cents {
	pctFee := math.Ceil(float64(total) * pricing.Card.Percent)
	return pricing.Card.Flat + currency.Cents(pctFee)
}

func calcPlatformAffFee(pricing pricing.Fees, total currency.Cents) currency.Cents {
	pctFee := math.Ceil(float64(total) * pricing.Affiliate.Percent)
	return pricing.Affiliate.Flat + currency.Cents(pctFee)
}

func calcAffiliateFee(comm commission.Commission, total currency.Cents) currency.Cents {
	pctFee := math.Floor(float64(total) * comm.Percent)
	return comm.Flat + currency.Cents(pctFee)
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

			// Create fake user to purchase some fake things
			usr := user.Fake(db)

			// Create some fake money for our fake user to spend
			pay := payment.Fake(db)

			// Create new authorization request
			req = new(checkout.Authorization)
			req.Order = ord
			req.Payment = pay
			req.User = usr

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

		It("Should parent order to user", func() {
			usr := getUser(res.UserId)
			getOrderByParent(usr.Key())
		})

		It("Should parent payment to order", func() {
			getPaymentByParent(res.Key())
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

			prod.Id_ = "FAKE_AND_BAD"

			li := lineitem.Fake(prod)
			ord := order.Fake(db, li)
			// Create new authorization request
			req = new(checkout.Authorization)
			req.Order = ord
			req.Payment = payment.Fake(db)
			req.User = user.Fake(db)
		})

		It("Should not authorize invalid product id", func() {
			cl.Post("/checkout/authorize", req, nil, 400)
		})
	})

	Context("Authorize invalid variant", func() {
		var req *checkout.Authorization
		Before(func() {

			// Create fake product, variant and order
			prod := product.Fake(db)
			prod.MustCreate()
			vari := variant.Fake(db, prod.Id())
			vari.MustCreate()
			vari.Id_ = "FAKE_AND_BAD"
			li := lineitem.Fake(vari)
			ord := order.Fake(db, li)

			// Create new authorization request
			req = new(checkout.Authorization)
			req.Order = ord
			req.Payment = payment.Fake(db)
			req.User = user.Fake(db)
		})
		It("Should not authorize invalid variant id", func() {
			cl.Post("/checkout/authorize", req, nil, 400)
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

		})

		JustBefore(func() {
			cl.Post("/checkout/authorize", req, res)
		})

		Context("User id used as id", func() {
			It("Should re-use user successfully", func() {
				Expect(res.UserId).To(Equal(usr.Id()))
			})

			It("Should allow email to be used as id", func() {
				Expect(res.UserId).To(Equal(usr.Id()))
			})
		})

		Context("User email used as id", func() {
			Before(func() {
				req.User.Id_ = req.User.Email
			})

			It("Should re-use user successfully", func() {
				Expect(res.UserId).To(Equal(usr.Id()))
			})

			It("Should allow email to be used as id", func() {
				Expect(res.UserId).To(Equal(usr.Id()))
			})
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
			ref = referrer.Fake(db, usr.Id())
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

		It("Should save platform fees", func() {
			pay := getPayment(res.Id())
			platformFee := calcPlatformFee(org.Fees, res.Total)

			fees := getFees(pay.Id(), "platform")
			Expect(len(fees)).To(Equal(1))
			Expect(fees[0].Amount).To(Equal(platformFee))
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
		})

		It("Should save referral for affiliate", func() {
			rfl := getReferral(res.Id())
			Expect(rfl.Referrer.AffiliateId).To(Equal(aff.Id()))
		})

		It("Should save platform fees", func() {
			pay := getPayment(res.Id())
			affFee := calcAffiliateFee(aff.Commission, res.Total)
			platformFee := calcPlatformFee(org.Fees, res.Total)
			platformAffFee := calcPlatformAffFee(org.Fees, affFee)

			fees := getFees(pay.Id(), "affiliate")
			Expect(len(fees)).To(Equal(1))
			Expect(fees[0].Amount).To(Equal(affFee))

			fees = getFees(pay.Id(), "platform")
			Expect(len(fees)).To(Equal(2))
			Expect(fees[0].Amount + fees[1].Amount).To(Equal(platformFee + platformAffFee))
		})
	})

	Context("Charge First Time Customers", func() {
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

			// Create fake user to purchase some fake things
			usr := user.Fake(db)

			// Create some fake money for our fake user to spend
			pay := payment.Fake(db)

			// Create new authorization request
			req = new(checkout.Authorization)
			req.Order = ord
			req.Payment = pay
			req.User = usr

			// Instantiate order to encompass result
			res = order.New(db)

			// Make request
			cl.Post("/checkout/charge", req, res)
		})
		It("Should save new order successfully", func() {
			getUser(res.UserId)
		})
		It("Should save new payment successfully", func() {
			getPayment(res.Id())
		})

		It("Should save new order successfully for store", func() {
			getOrder(res.Id())
		})

		It("Should parent order to user", func() {
			usr := getUser(res.UserId)
			getOrderByParent(usr.Key())
		})

		It("Should parent payment to order", func() {
			getPaymentByParent(res.Key())
		})

		It("Should save payment id on order", func() {
			Expect(len(res.PaymentIds)).To(Equal(1))
		})

		It("Should calculate correct total for order and payment", func() {
			Expect(res.Total).To(Equal(req.Order.Total))
		})
	})

	Context("Charge invalid product", func() {
		var req *checkout.Authorization

		Before(func() {
			// Create fake product, variant and order
			prod := product.Fake(db)
			prod.MustCreate()

			prod.Id_ = "FAKE_AND_BAD"

			li := lineitem.Fake(prod)
			ord := order.Fake(db, li)
			// Create new authorization request
			req = new(checkout.Authorization)
			req.Order = ord
			req.Payment = payment.Fake(db)
			req.User = user.Fake(db)
		})

		It("Should not charge invalid product id", func() {
			cl.Post("/checkout/charge", req, nil, 400)
		})
	})

	Context("Charge invalid variant", func() {
		var req *checkout.Authorization
		Before(func() {

			// Create fake product, variant and order
			prod := product.Fake(db)
			prod.MustCreate()
			vari := variant.Fake(db, prod.Id())
			vari.MustCreate()
			vari.Id_ = "FAKE_AND_BAD"
			li := lineitem.Fake(vari)
			ord := order.Fake(db, li)

			// Create new authorization request
			req = new(checkout.Authorization)
			req.Order = ord
			req.Payment = payment.Fake(db)
			req.User = user.Fake(db)
		})

		It("Should not charge invalid variant id", func() {
			cl.Post("/checkout/charge", req, nil, 400)
		})

		It("Should not charge invalid collection id", func() {
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
