package stripe_test

import (
	"fmt"
	"strings"

	"crowdstart.com/models/coupon"
	"crowdstart.com/models/lineitem"
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/store"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/user"
	"crowdstart.com/test/api/checkout/integration/requests"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"

	. "crowdstart.com/util/test/ginkgo"
)

type testHelperReturn struct {
	Payments []*payment.Payment
	Orders   []*order.Order
}

func FirstTimeSuccessfulOrderTest(isCharge bool, stor *store.Store) testHelperReturn {
	var path string
	if isCharge {
		path = "/charge"
	} else {
		path = "/authorize"
	}

	if stor != nil {
		path = "/store/" + stor.Id() + path
	}
	log.Warn(path)

	// Should come back with 200
	w := cl.Post(path, requests.ValidOrder, nil)
	Expect(w.Code).To(Equal(200))

	log.Debug("JSON %v", w.Body)

	// Payment and Order info should be in the dv
	ord := order.New(db)

	err := json.DecodeBuffer(w.Body, &ord)
	Expect(err).ToNot(HaveOccurred())

	log.Debug("Order %v", ord)

	// Order should be in db
	key, _, err := order.New(db).KeyExists(ord.Id())
	log.Debug("Err %v", err)

	Expect(err).ToNot(HaveOccurred())
	Expect(key).ToNot(BeNil())

	// User should be in db
	usr := user.New(db)
	err = usr.Get(ord.UserId)

	Expect(err).ToNot(HaveOccurred())
	Expect(usr.Key()).ToNot(BeNil())
	stripeVerifyUser(usr)

	// Payment should be in db
	Expect(len(ord.PaymentIds)).To(Equal(1))

	pay := payment.New(db)
	err = pay.Get(ord.PaymentIds[0])

	Expect(err).ToNot(HaveOccurred())
	Expect(pay.Key()).ToNot(BeNil())

	if isCharge {
		stripeVerifyCharge(pay)
	} else {
		stripeVerifyAuth(pay)
	}

	stripeVerifyCards(usr, []string{pay.Account.CardId})

	return testHelperReturn{
		Payments: []*payment.Payment{pay},
		Orders:   []*order.Order{ord},
	}
}

func ReturningSuccessfulOrderSameCardTest(isCharge bool, stor *store.Store) testHelperReturn {
	var path string
	if isCharge {
		path = "/charge"
	} else {
		path = "/authorize"
	}

	if stor != nil {
		path = "/store/" + stor.Id() + path
	}

	// Make first request
	ord1 := order.New(db)
	cl.Post(path, requests.ValidOrder, ord1)

	// Fetch the payment for the order to test later
	pay1 := payment.New(db)
	pay1.Get(ord1.PaymentIds[0])
	if isCharge {
		stripeVerifyCharge(pay1)
	} else {
		stripeVerifyAuth(pay1)
	}

	// Save user, customerId from first order
	usr := user.New(db)
	usr.Get(ord1.UserId)

	log.JSON("USERRRRRRRRRRRRRRRRRRRR", usr)

	customerId := usr.Accounts.Stripe.CustomerId
	stripeVerifyUser(usr)

	// Returning user, should reuse stripe customer id
	body := fmt.Sprintf(requests.ReturningUserOrder, usr.Id())
	ord2 := order.New(db)
	cl.Post(path, body, ord2)

	Expect(usr.Id()).To(Equal(ord2.UserId))

	// Fetch the payment for the order to test later
	pay2 := payment.New(db)
	pay2.Get(ord2.PaymentIds[0])
	if isCharge {
		stripeVerifyCharge(pay2)
	} else {
		stripeVerifyAuth(pay2)
	}

	user2 := user.New(db)
	user2.Get(ord2.UserId)
	Expect(user2.Accounts.Stripe.CustomerId).To(Equal(customerId))

	Expect(pay1.Account.CardId).ToNot(Equal(pay2.Account.CardId))
	stripeVerifyCards(usr, []string{pay2.Account.CardId})

	return testHelperReturn{
		Payments: []*payment.Payment{pay1, pay2},
		Orders:   []*order.Order{ord1, ord2},
	}
}

func ReturningSuccessfulOrderNewCardTest(isCharge bool, stor *store.Store) testHelperReturn {
	var path string
	if isCharge {
		path = "/charge"
	} else {
		path = "/authorize"
	}

	if stor != nil {
		path = "/store/" + stor.Id() + path
	}

	// Make first request
	ord1 := order.New(db)
	cl.Post(path, requests.ValidOrder, ord1)

	// Fetch the payment for the order to test later
	pay1 := payment.New(db)
	pay1.Get(ord1.PaymentIds[0])
	if isCharge {
		stripeVerifyCharge(pay1)
	} else {
		stripeVerifyAuth(pay1)
	}

	// Save user, customerId from first order
	usr := user.New(db)
	usr.Get(ord1.UserId)
	customerId := usr.Accounts.Stripe.CustomerId
	stripeVerifyUser(usr)

	// Returning user, should reuse stripe customer id
	body := fmt.Sprintf(requests.ReturningUserOrderNewCard, usr.Id())
	ord2 := order.New(db)
	cl.Post(path, body, ord2)
	Expect(usr.Id()).To(Equal(ord2.UserId))

	// Fetch the payment for the order to test later
	pay2 := payment.New(db)
	pay2.Get(ord2.PaymentIds[0])
	if isCharge {
		stripeVerifyCharge(pay2)
	} else {
		stripeVerifyAuth(pay2)
	}

	user2 := user.New(db)
	user2.Get(ord2.UserId)
	Expect(user2.Accounts.Stripe.CustomerId).To(Equal(customerId))

	// Payment/Card logic
	Expect(pay1.Account.CardId).ToNot(Equal(pay2.Account.CardId))
	stripeVerifyCards(usr, []string{pay1.Account.CardId, pay2.Account.CardId})

	return testHelperReturn{
		Payments: []*payment.Payment{pay1, pay2},
		Orders:   []*order.Order{ord1, ord2},
	}
}

func OrderBadCardTest(isCharge bool, stor *store.Store) {
	var path string
	if isCharge {
		path = "/charge"
	} else {
		path = "/authorize"
	}

	if stor != nil {
		path = "/store/" + stor.Id() + path
	}

	// Returning user, should reuse stripe customer id
	body := fmt.Sprintf(requests.InvalidOrderBadCard)
	cl.Post(path, body, nil, 402)
}

func OrderBadUserTest(isCharge bool, stor *store.Store) {
	var path string
	if isCharge {
		path = "/charge"
	} else {
		path = "/authorize"
	}

	if stor != nil {
		path = "/store/" + stor.Id() + path
	}

	// Returning user, should reuse stripe customer id
	body := fmt.Sprintf(requests.ReturningUserOrderNewCard, "BadId")
	cl.Post(path, body, nil, 400)
}

var _ = Describe("payment", func() {
	Context("Authorize First Time Customers", func() {
		It("Should normalise the user information", func() {
			ord := order.New(db)
			cl.Post("/checkout/authorize", requests.NonNormalizedOrder, ord)

			usr := user.New(db)
			usr.Get(ord.UserId)

			log.JSON("USER:", usr)

			var normalize = func(s string) string {
				return strings.ToLower(strings.TrimSpace(s))
			}

			Expect(usr.Username).To(Equal(normalize(usr.Username)))
			Expect(usr.Email).To(Equal(normalize(usr.Email)))
			Expect(ord.BillingAddress.Country).To(Equal(strings.ToUpper(ord.BillingAddress.Country)))
			Expect(ord.ShippingAddress.Country).To(Equal(strings.ToUpper(ord.ShippingAddress.Country)))
		})

		It("Should save new order successfully", func() {
			FirstTimeSuccessfulOrderTest(false, nil)
		})

		It("Should save new order successfully for store", func() {
			FirstTimeSuccessfulOrderTest(false, stor)
		})

		It("Should not authorize invalid credit card number", func() {
			OrderBadCardTest(false, nil)
		})

		It("Should not authorize invalid credit card number for store", func() {
			OrderBadCardTest(false, stor)
		})
		// It("Should not authorize invalid product id", func() {
		// })
		// It("Should not authorize invalid variant id", func() {
		// })
		// It("Should not authorize invalid collection id", func() {
		// })
	})

	FContext("Authorize Returning Customers", func() {
		It("Should save returning customer order with the same card successfully", func() {
			ReturningSuccessfulOrderSameCardTest(false, nil)
		})

		It("Should save returning customer order with the same card successfully for store", func() {
			ReturningSuccessfulOrderSameCardTest(false, stor)
		})

		It("Should save returning customer order with a new card successfully", func() {
			ReturningSuccessfulOrderNewCardTest(false, nil)
		})

		It("Should save returning customer order with a new card successfully for store", func() {
			ReturningSuccessfulOrderNewCardTest(false, stor)
		})

		It("Should not save customer with invalid user id", func() {
			OrderBadUserTest(false, nil)
		})

		It("Should not save customer with invalid user id for store", func() {
			OrderBadUserTest(false, stor)
		})
	})

	Context("Charge First Time Customers", func() {
		It("Should save new order successfully", func() {
			FirstTimeSuccessfulOrderTest(true, nil)
		})

		It("Should save new order successfully for store", func() {
			FirstTimeSuccessfulOrderTest(true, stor)
		})

		It("Should not authorize invalid credit card number", func() {
			OrderBadCardTest(true, nil)
		})

		It("Should not authorize invalid credit card number for store", func() {
			OrderBadCardTest(true, stor)
		})

		// It("Should not authorize invalid product id", func() {
		// })
		// It("Should not authorize invalid variant id", func() {
		// })
		// It("Should not authorize invalid collection id", func() {
		// })
	})

	Context("Charge Returning Customers", func() {
		It("Should save returning customer order with the same card successfully", func() {
			ReturningSuccessfulOrderSameCardTest(true, nil)
		})

		It("Should save returning customer order with the same card successfully for store", func() {
			ReturningSuccessfulOrderSameCardTest(true, stor)
		})

		It("Should save returning customer order with a new card successfully", func() {
			ReturningSuccessfulOrderNewCardTest(true, nil)
		})

		It("Should save returning customer order with a new card successfully for store", func() {
			ReturningSuccessfulOrderNewCardTest(true, stor)
		})

		It("Should not save customer with invalid user id", func() {
			OrderBadUserTest(true, nil)
		})

		It("Should not save customer with invalid user id", func() {
			OrderBadUserTest(true, stor)
		})
	})

	Context("Authorize Order", func() {
		It("Should authorize existing order successfully", func() {
			ord1 := order.New(db)
			cl.Post("/order", requests.ValidOrderOnly, ord1, 201)

			ord2 := order.New(db)
			err := ord2.Get(ord1.Id())
			Expect(err).ToNot(HaveOccurred())

			ord3 := order.New(db)
			cl.Post("/order/"+ord2.Id()+"/authorize", requests.ValidUserPaymentOnly, ord3)

			pay := payment.New(db)
			pay.Get(ord3.PaymentIds[0])

			stripeVerifyAuth(pay)
		})

		It("Should not capture invalid order", func() {
			cl.Post("/order/BADID/authorize", "", nil, 404)
		})

		It("Should authorize order with coupon successfully", func() {
			ord1 := order.New(db)
			cl.Post("/order", requests.ValidCouponOrderOnly, ord1, 201)

			ord2 := order.New(db)
			err := ord2.Get(ord1.Id())
			Expect(err).ToNot(HaveOccurred())

			ord3 := order.New(db)
			cl.Post("/order/"+ord2.Id()+"/authorize", requests.ValidUserPaymentOnly, ord3)

			Expect(ord3.Subtotal).To(Equal(currency.Cents(3500)))

			pay := payment.New(db)
			pay.Get(ord3.PaymentIds[0])

			stripeVerifyAuth(pay)
		})
	})

	Context("Capture Order", func() {
		It("Should capture existing authorized order successfully", func() {
			pnos := FirstTimeSuccessfulOrderTest(false, nil)
			id := pnos.Orders[0].Id()

			cl.Post("/order/"+id+"/capture", "", nil)
			stripeVerifyCharge(pnos.Payments[0])
		})

		It("Should not capture invalid order", func() {
			cl.Post("/order/BADID/capture", "", nil, 404)
		})
	})

	Context("Charge Order", func() {
		It("Should charge existing order successfully", func() {
			ord1 := order.New(db)
			cl.Post("/order", requests.ValidOrderOnly, ord1, 201)

			ord2 := order.New(db)
			err := ord2.Get(ord1.Id())
			Expect(err).ToNot(HaveOccurred())

			ord3 := order.New(db)
			cl.Post("/order/"+ord2.Id()+"/charge", requests.ValidUserPaymentOnly, ord3)

			pay := payment.New(db)
			pay.Get(ord3.PaymentIds[0])

			stripeVerifyCharge(pay)
		})

		It("Should not capture invalid order", func() {
			cl.Post("/order/BADID/charge", "", nil, 404)
		})
	})

	Context("Charge Order With Referral", func() {
		It("Should charge existing order with referral successfully", func() {
			ord1 := order.New(db)
			ord1.UserId = u.Id()
			ord1.Currency = currency.USD
			ord1.ReferrerId = refIn.Id()
			ord1.Items = []lineitem.LineItem{
				lineitem.LineItem{
					ProductId: prod.Id(),
					Quantity:  1,
				},
			}
			err := ord1.Put()
			Expect(err).ToNot(HaveOccurred())

			ord2 := order.New(db)
			cl.Post("/order/"+ord1.Id()+"/charge", requests.ValidUserPaymentOnly, ord2)

			refIn1 := referrer.New(db)
			refIn1.MustGet(refIn.Id())

			// trans := transaction.New(db)
			// err = trans.GetById(refIn1.TransactionIds[0])
			// Expect(err).ToNot(HaveOccurred())
			// Expect(trans.UserId).To(Equal(u.Id()))
			// Expect(trans.Currency).To(Equal(refIn.Program.Actions[0].Currency))
			// Expect(trans.Amount).To(Equal(refIn.Program.Actions[0].Amount))

			pay := payment.New(db)
			pay.Get(ord2.PaymentIds[0])

			stripeVerifyCharge(pay)
		})
	})

	Context("Charge Order With Single Use Coupon", func() {
		It("Should charge order with single use coupon successfully", func() {
			Skip("Single-use coupons not yet supported")

			ord := order.New(db)
			cl.Post("/checkout/charge", requests.ValidOrder, ord)

			cpn := coupon.New(db)
			cl.Get("/coupon/no-doge-left-behind/code/"+u.Id(), cpn)

			jsonStr := fmt.Sprintf(requests.ValidOrderTemplate, ord.UserId, cpn.Code())
			ord2 := order.New(db)
			cl.Post("/checkout/charge", jsonStr, ord2)

			Expect(ord2.Items[1].ProductSlug).To(Equal("doge-shirt"))

			jsonStr = fmt.Sprintf(requests.ValidOrderTemplate, ord.UserId, cpn.Code())
			cl.Post("/checkout/charge", jsonStr, nil, 400)
		})
	})

	Context("Refund Order", func() {
		It("Should refund order successfully", func() {
			ord1 := order.Fake(db, lineitem.LineItem{
				ProductId: prod.Id(),
				Quantity:  1,
			})

			ord1.MustCreate()
			ordId := ord1.Id()

			cl.Post("/order/"+ordId+"/charge", requests.ValidUserPaymentOnly, nil, 200)
			cl.Post("/order/"+ordId+"/refund", requests.NegativeRefund, nil, 400)
			cl.Post("/order/"+ordId+"/refund", requests.LargeRefundAmount, nil, 400)
			cl.Post("/order/"+ordId+"/refund", requests.PartialRefund, nil, 200)

			refundedOrder := order.New(db)
			err := refundedOrder.Get(ordId)
			Expect(err).ToNot(HaveOccurred())
			Expect(refundedOrder.Refunded).To(Equal(currency.Cents(123)))

			payments, err := refundedOrder.GetPayments()
			Expect(err).ToNot(HaveOccurred())
			for _, p := range payments {
				if p.AmountRefunded == p.Amount {
					Expect(string(p.Status)).To(Equal(payment.Refunded))
				} else {
					Expect(string(p.Status)).To(Equal(payment.Paid))
				}
			}
		})
	})

	// Other things that could be tested
	// Capturing an unauthorized order
	// Capturing a captured order
	// Authorizing a captured order
})
