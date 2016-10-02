package test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/coupon"
	"crowdstart.com/models/fixtures"
	"crowdstart.com/models/lineitem"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/product"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/store"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/user"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/gincontext"
	"crowdstart.com/util/hashid"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/test/ae"
	"crowdstart.com/util/test/ginclient"

	checkoutApi "crowdstart.com/api/checkout"
	couponApi "crowdstart.com/api/coupon"
	orderApi "crowdstart.com/api/order"
	storeApi "crowdstart.com/api/store"

	. "crowdstart.com/test/api/checkout/requests"
	. "crowdstart.com/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("api/checkout", t)
}

var (
	ctx         ae.Context
	client      *ginclient.Client
	accessToken string
	db          *datastore.Datastore
	org         *organization.Organization
	prod        *product.Product
	stor        *store.Store
	sc          *stripe.Client
	u           *user.User
	refIn       *referrer.Referrer
)

// Setup appengine context
var _ = BeforeSuite(func() {
	adminRequired := middleware.TokenRequired(permission.Admin)

	ctx = ae.NewContext()

	// Mock gin context that we can use with fixtures
	c := gincontext.New(ctx)

	// Run default fixtures to setup organization and store, etc
	u = fixtures.User(c).(*user.User)
	org = fixtures.Organization(c).(*organization.Organization)
	stor = fixtures.Store(c).(*store.Store)
	prod = fixtures.Product(c).(*product.Product)
	fixtures.Variant(c)
	fixtures.Coupon(c)
	fixtures.Discount(c)
	refIn = fixtures.Referrer(c).(*referrer.Referrer)

	// Setup client and add routes for payment API tests.
	client = ginclient.New(ctx)
	checkoutApi.Route(client.Router, adminRequired)
	orderApi.Route(client.Router, adminRequired)
	storeApi.Route(client.Router, adminRequired)
	couponApi.Route(client.Router, adminRequired)

	// Create organization for tests, accessToken
	accessToken = org.AddToken("test-published-key", permission.Admin)
	org.MustPut()

	// Set authorization header for subsequent requests
	client.Defaults(func(r *http.Request) {
		r.Header.Set("Authorization", accessToken)
	})

	// Stripe client
	sc = stripe.New(ctx, org.Stripe.Test.AccessToken)

	// Save namespaced db
	db = datastore.New(org.Namespaced(ctx))
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

type testHelperReturn struct {
	Payments []*payment.Payment
	Orders   []*order.Order
}

func post(path, req string, dst interface{}, args ...interface{}) *httptest.ResponseRecorder {
	w := client.PostJSON(path, req)

	switch len(args) {
	case 0:
		Expect1(w.Code < 400).To(BeTrue())
	case 1:
		Expect1(w.Code == args[0]).To(BeTrue())
	default:
		panic("Takes optional status code only")
	}

	err := json.DecodeBuffer(w.Body, dst)
	Expect1(err).ToNot(HaveOccurred())
	return w
}

func keyExists(key string) {
	ok, err := hashid.KeyExists(db.Context, key)
	Expect1(err).ToNot(HaveOccurred())
	Expect1(ok).To(BeTrue())
}

func getOrder(id string) *order.Order {
	ord := order.New(db)
	err := ord.Get(id)
	Expect1(err).ToNot(HaveOccurred())
	return ord
}

func getUser(id string) *user.User {
	usr := user.New(db)
	err := usr.Get(id)
	Expect1(err).ToNot(HaveOccurred())
	return usr
}

func getPayment(id string) *payment.Payment {
	pay := payment.New(db)
	err := pay.Get(id)
	Expect1(err).ToNot(HaveOccurred())
	return pay
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

	// Should come back with 200
	ord := order.New(db)
	post(path, ValidOrder, ord)

	// Order should be in db
	keyExists(ord.Id())

	// User should be in db
	keyExists(ord.UserId)

	usr := user.New(db)
	usr.Get(ord.Id())
	stripeVerifyUser(usr)

	// Payment should be in db
	Expect1(len(ord.PaymentIds)).To(Equal(1))
	keyExists(ord.PaymentIds[0])

	pay := payment.New(db)
	pay.Get(ord.PaymentIds[0])

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
	post(path, ValidOrder, ord1)

	// Fetch the payment for the order to test later
	pay1 := payment.New(db)
	pay1.Get(ord1.PaymentIds[0])
	if isCharge {
		stripeVerifyCharge(pay1)
	} else {
		stripeVerifyAuth(pay1)
	}

	// Save user, customerId from first order
	post(path, ReturningUserOrder(usr.Id()), ord1)
	usr := getUser(ord1.UserId)
	customerId := usr.Accounts.Stripe.CustomerId
	stripeVerifyUser(usr)

	// Returning user, should reuse stripe customer id
	ord2 := order.New(db)
	post(path, ReturningUserOrder(usr.Id()), ord2)

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
	w := client.PostJSON(path, ValidOrder)
	Expect(w.Code).To(Equal(200))
	log.Debug("JSON %v", w.Body)

	// Decode body so we can re-use user id
	ord1 := order.New(db)
	err := json.DecodeBuffer(w.Body, &ord1)
	Expect(err).ToNot(HaveOccurred())

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
	body := fmt.Sprintf(ReturningUserOrderNewCard, usr.Id())
	log.Debug("JSON %v", w.Body)
	w = client.PostJSON(path, body)
	Expect(w.Code).To(Equal(200))

	// Decode body from second request
	ord2 := order.New(db)
	err = json.DecodeBuffer(w.Body, &ord2)
	Expect(err).ToNot(HaveOccurred())
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
	body := fmt.Sprintf(InvalidOrderBadCard)
	w := client.PostJSON(path, body)
	log.Debug("JSON %v", w.Body)
	Expect(w.Code).To(Equal(402))
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
	body := fmt.Sprintf(ReturningUserOrderNewCard, "BadId")
	w := client.PostJSON(path, body)
	log.Debug("JSON %v", w.Body)
	Expect(w.Code).To(Equal(400))
}

var _ = Describe("payment", func() {
	Context("Authorize First Time Customers", func() {
		It("Should normalise the user information", func() {
			path := "/order"
			w := client.PostJSON(path, NonNormalizedOrder)

			ord := order.New(db)
			json.DecodeBuffer(w.Body, &ord)

			usr := user.New(db)
			usr.Get(ord.UserId)

			var normalize = func(s string) string {
				return strings.ToLower(strings.TrimSpace(s))
			}

			Expect(usr.Username).To(Equal(normalize(usr.Username)))
			Expect(usr.Email).To(Equal(normalize(usr.Email)))
			Expect(ord.BillingAddress.Country).To(Equal(strings.ToUpper(ord.BillingAddress.Country)))
			Expect(ord.ShippingAddress.Country).To(Equal(strings.ToUpper(ord.ShippingAddress.Country)))
		})

		FIt("Should authorize new order successfully", func() {
			ord := decodeOrder(client.PostJSON("/authorize", ValidOrder))

			// Order should be in db
			keyExists(ord.Id())

			// User should be in db
			usr := getUser(ord.UserId)
			stripeVerifyUser(usr)

			// Payment should be in db
			Expect(len(ord.PaymentIds)).To(Equal(1))
			pay := getPayment(ord.PaymentIds[0])

			stripeVerifyAuth(pay)
			stripeVerifyCards(usr, []string{pay.Account.CardId})
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

	Context("Authorize Returning Customers", func() {
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
			// Create new order
			ord1 := order.New(db)
			post("/order", ValidOrderOnly, ord1)

			// Ensure in db
			ord2 := getOrder(ord1.Id())

			// Authorize order
			ord3 := order.New(db)
			post("/order/"+ord2.Id()+"/authorize", ValidUserPaymentOnly, ord3)

			// Verify payment exists in stripe
			pay := getPayment(ord3.PaymentIds[0])
			stripeVerifyAuth(pay)
		})

		It("Should not authorize invalid order", func() {
			post("/order/BADID/authorize", nil, nil)
		})

		It("Should authorize order with coupon successfully", func() {
			w := client.PostJSON("/order", ValidCouponOrderOnly)
			Expect(w.Code).To(Equal(201))

			ord1 := order.New(db)
			err := json.DecodeBuffer(w.Body, &ord1)
			Expect(err).ToNot(HaveOccurred())

			ord2 := order.New(db)
			err = ord2.Get(ord1.Id())
			Expect(err).ToNot(HaveOccurred())

			w = client.PostJSON("/order/"+ord2.Id()+"/authorize", ValidUserPaymentOnly)
			Expect(w.Code).To(Equal(200))
			log.Debug("JSON %v", w.Body)

			ord3 := order.New(db)
			err = json.DecodeBuffer(w.Body, &ord3)
			Expect(err).ToNot(HaveOccurred())
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

			w := client.PostJSON("/order/"+id+"/capture", "")
			Expect(w.Code).To(Equal(200))
			log.Debug("JSON %v", w.Body)
			stripeVerifyCharge(pnos.Payments[0])
		})

		It("Should not capture invalid order", func() {
			w := client.PostJSON("/order/BADID/capture", "")
			Expect(w.Code).To(Equal(404))
			log.Debug("JSON %v", w.Body)
		})
	})

	Context("Charge Order", func() {
		It("Should charge existing order successfully", func() {
			w := client.PostJSON("/order", ValidOrderOnly)
			Expect(w.Code).To(Equal(201))

			ord1 := order.New(db)
			err := json.DecodeBuffer(w.Body, &ord1)
			Expect(err).ToNot(HaveOccurred())

			ord2 := order.New(db)
			err = ord2.Get(ord1.Id())
			Expect(err).ToNot(HaveOccurred())

			w = client.PostJSON("/order/"+ord2.Id()+"/charge", ValidUserPaymentOnly)
			Expect(w.Code).To(Equal(200))
			log.Debug("JSON %v", w.Body)

			ord3 := order.New(db)
			err = json.DecodeBuffer(w.Body, &ord3)
			Expect(err).ToNot(HaveOccurred())

			pay := payment.New(db)
			pay.Get(ord3.PaymentIds[0])

			stripeVerifyCharge(pay)
		})

		It("Should not capture invalid order", func() {
			w := client.PostJSON("/order/BADID/charge", "")
			Expect(w.Code).To(Equal(404))
			log.Debug("JSON %v", w.Body)
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

			w := client.PostJSON("/order/"+ord1.Id()+"/charge", ValidUserPaymentOnly)
			Expect(w.Code).To(Equal(200))
			log.Debug("JSON %v", w.Body)

			referrals, err := refIn.Referrals()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(referrals)).To(Equal(0))

			transactions, err := refIn.Transactions()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(transactions)).To(Equal(0))

			refIn1 := referrer.New(db)
			refIn1.MustGet(refIn.Id())

			referrals, err = refIn1.Referrals()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(referrals)).To(Equal(1))

			transactions, err = refIn1.Transactions()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(transactions)).To(Equal(1))

			trans := transactions[0]
			Expect(err).ToNot(HaveOccurred())
			Expect(trans.UserId).To(Equal(u.Id()))
			Expect(trans.Currency).To(Equal(refIn.Program.Actions[0].Currency))
			Expect(trans.Amount).To(Equal(refIn.Program.Actions[0].Amount))

			ord2 := order.New(db)
			err = json.DecodeBuffer(w.Body, &ord2)
			Expect(err).ToNot(HaveOccurred())

			pay := payment.New(db)
			pay.Get(ord2.PaymentIds[0])

			stripeVerifyCharge(pay)
		})
	})

	Context("Charge Order With Single Use Coupon", func() {
		It("Should charge order with single use coupon successfully", func() {
			Skip("Single-use coupons not yet supported")
			w := client.PostJSON("/checkout/charge", ValidOrder)
			Expect(w.Code).To(Equal(200))

			ord := order.New(db)
			err := json.DecodeBuffer(w.Body, ord)
			Expect(err).ToNot(HaveOccurred())

			w = client.Get("/coupon/no-doge-left-behind/code/" + u.Id())
			Expect(w.Code).To(Equal(200))
			log.Debug("JSON %v", w.Body)

			cpn := coupon.New(db)
			err = json.DecodeBuffer(w.Body, &cpn)
			Expect(err).ToNot(HaveOccurred())

			jsonStr := fmt.Sprintf(ValidOrderTemplate, ord.UserId, cpn.Code())
			w = client.PostJSON("/checkout/charge", jsonStr)
			Expect(w.Code).To(Equal(200))
			log.Debug("JSON %v", w.Body)

			ord2 := order.New(db)
			err = json.DecodeBuffer(w.Body, ord2)
			Expect(err).ToNot(HaveOccurred())

			Expect(ord2.Items[1].ProductSlug).To(Equal("doge-shirt"))

			jsonStr = fmt.Sprintf(ValidOrderTemplate, ord.UserId, cpn.Code())
			w = client.PostJSON("/checkout/charge", jsonStr)
			Expect(w.Code).To(Equal(400))
			log.Debug("JSON %v", w.Body)
		})
	})

	Context("Charge Order With Discount Rules Applicable", func() {
		It("Should charge order and apply appropriate discount rules", func() {
			ord := order.New(db)

			post("/checkout/charge", DiscountO)
			jsonStr := fmt.Sprintf(DiscountOrderTemplate, "batman-shirt")
			w := client.PostJSON("/checkout/charge", jsonStr)
			decodeOrder(w)

			jsonStr = fmt.Sprintf(DiscountOrderTemplate, prod.Id())
			w = client.PostJSON("/checkout/charge", jsonStr)
			ord := decodeOrder(w)

			jsonStr = fmt.Sprintf(ValidOrderTemplate, ord.UserId, "NO-DOGE-LEFT-BEHIND")
			w = client.PostJSON("/checkout/charge", jsonStr)
			decodeOrder(w)
		})
	})

	Context("Refund Order", func() {
		It("Should refund order successfully", func() {
			ord1 := order.New(db)
			ord1.UserId = u.Id()
			ord1.Currency = currency.USD
			ord1.Items = []lineitem.LineItem{
				lineitem.LineItem{
					ProductId: prod.Id(),
					Quantity:  1,
				},
			}
			err := ord1.Put()
			Expect(err).ToNot(HaveOccurred())
			ordId := ord1.Id()

			w := client.PostJSON("/order/"+ordId+"/charge", ValidUserPaymentOnly)
			Expect(w.Code).To(Equal(200))
			log.Debug("JSON %v", w.Body)

			w = client.PostJSON("/order/"+ordId+"/refund", NegativeRefund)
			Expect(w.Code).ToNot(Equal(200))

			w = client.PostJSON("/order/"+ordId+"/refund", LargeRefundAmount)
			Expect(w.Code).ToNot(Equal(200))

			w = client.PostJSON("/order/"+ordId+"/refund", PartialRefund)
			Expect(w.Code).To(Equal(200))

			refundedOrder := order.New(db)
			err = refundedOrder.Get(ordId)
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
