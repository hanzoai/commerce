package test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"golang.org/x/net/context"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/app"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/lineitem"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/product"
	"hanzo.io/models/referral"
	"hanzo.io/models/referrer"
	"hanzo.io/models/store"
	"hanzo.io/models/transaction"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/user"
	"hanzo.io/test/api/checkout/requests"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/json"
	"hanzo.io/util/log"
	"hanzo.io/util/permission"
	"hanzo.io/util/test/ae"
	"hanzo.io/util/test/ginclient"

	checkoutApi "hanzo.io/api/checkout"
	orderApi "hanzo.io/api/order"
	storeApi "hanzo.io/api/store"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("api/checkout/stripe_integration", t)
}

var (
	ap     *app.App
	apiKey string
	client *ginclient.Client
	ctx    context.Context
	inst   ae.Instance
	org    *organization.Organization
	Org    *organization.Organization
	prod   *product.Product
	stor   *store.Store
	u      *user.User
	refIn  *referrer.Referrer
)

type testHelperReturn struct {
	Payments []*payment.Payment
	Orders   []*order.Order
}

func FirstTimeSuccessfulOrderTest(isCharge bool, stor *store.Store) testHelperReturn {
	db := datastore.New(org.Namespaced(ctx))

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
	w := client.PostRawJSON(path, requests.ValidOrder)
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
	db := datastore.New(org.Namespaced(ctx))

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
	w := client.PostRawJSON(path, requests.ValidOrder)
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
	body := fmt.Sprintf(requests.ReturningUserOrder, usr.Id())
	log.Debug("JSON %v", w.Body)
	w = client.PostRawJSON(path, body)
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
	Expect(pay1.Account.CardId).To(Equal(pay2.Account.CardId))
	stripeVerifyCards(usr, []string{pay1.Account.CardId})

	return testHelperReturn{
		Payments: []*payment.Payment{pay1, pay2},
		Orders:   []*order.Order{ord1, ord2},
	}
}

func ReturningSuccessfulOrderNewCardTest(isCharge bool, stor *store.Store) testHelperReturn {
	db := datastore.New(org.Namespaced(ctx))

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
	w := client.PostRawJSON(path, requests.ValidOrder)
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
	body := fmt.Sprintf(requests.ReturningUserOrderNewCard, usr.Id())
	log.Debug("JSON %v", w.Body)
	w = client.PostRawJSON(path, body)
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
	body := fmt.Sprintf(requests.InvalidOrderBadCard)
	w := client.PostRawJSON(path, body)
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
	body := fmt.Sprintf(requests.ReturningUserOrderNewCard, "BadId")
	w := client.PostRawJSON(path, body)
	log.Debug("JSON %v", w.Body)
	Expect(w.Code).To(Equal(500))
}

var _ = Describe("checkout", func() {
	Context("Authorize First Time Customers", func() {
		It("Should normalize the user information", func() {
			db := datastore.New(org.Namespaced(ctx))

			path := "/order"
			w := client.PostRawJSON(path, requests.NonNormalizedOrder)

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
			db := datastore.New(org.Namespaced(ctx))

			w := client.PostRawJSON("/order", requests.ValidOrderOnly)
			Expect(w.Code).To(Equal(201))

			ord1 := order.New(db)
			err := json.DecodeBuffer(w.Body, &ord1)
			Expect(err).ToNot(HaveOccurred())

			ord2 := order.New(db)
			err = ord2.Get(ord1.Id())
			Expect(err).ToNot(HaveOccurred())

			w = client.PostRawJSON("/order/"+ord2.Id()+"/authorize", requests.ValidUserPaymentOnly)
			Expect(w.Code).To(Equal(200))
			log.Debug("JSON %v", w.Body)

			ord3 := order.New(db)
			err = json.DecodeBuffer(w.Body, &ord3)
			Expect(err).ToNot(HaveOccurred())

			pay := payment.New(db)
			pay.Get(ord3.PaymentIds[0])

			stripeVerifyAuth(pay)
		})

		It("Should not capture invalid order", func() {
			w := client.PostRawJSON("/order/BADID/authorize", "")
			Expect(w.Code).To(Equal(404))
			log.Debug("JSON %v", w.Body)
		})

		It("Should authorize order with coupon successfully", func() {
			db := datastore.New(org.Namespaced(ctx))

			w := client.PostRawJSON("/order", requests.ValidCouponOrderOnly)
			Expect(w.Code).To(Equal(201))

			ord1 := order.New(db)
			err := json.DecodeBuffer(w.Body, &ord1)
			Expect(err).ToNot(HaveOccurred())

			ord2 := order.New(db)
			err = ord2.Get(ord1.Id())
			Expect(err).ToNot(HaveOccurred())

			w = client.PostRawJSON("/order/"+ord2.Id()+"/authorize", requests.ValidUserPaymentOnly)
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

			w := client.PostRawJSON("/order/"+id+"/capture", "")
			Expect(w.Code).To(Equal(200))
			log.Debug("JSON %v", w.Body)
			stripeVerifyCharge(pnos.Payments[0])
		})

		It("Should not capture invalid order", func() {
			w := client.PostRawJSON("/order/BADID/capture", "")
			Expect(w.Code).To(Equal(404))
			log.Debug("JSON %v", w.Body)
		})
	})

	Context("Charge Order", func() {
		It("Should charge existing order successfully", func() {
			db := datastore.New(org.Namespaced(ctx))

			w := client.PostRawJSON("/order", requests.ValidOrderOnly)
			Expect(w.Code).To(Equal(201))

			ord1 := order.New(db)
			err := json.DecodeBuffer(w.Body, &ord1)
			Expect(err).ToNot(HaveOccurred())

			ord2 := order.New(db)
			err = ord2.Get(ord1.Id())
			Expect(err).ToNot(HaveOccurred())

			w = client.PostRawJSON("/order/"+ord2.Id()+"/charge", requests.ValidUserPaymentOnly)
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
			w := client.PostRawJSON("/order/BADID/charge", "")
			Expect(w.Code).To(Equal(404))
			log.Debug("JSON %v", w.Body)
		})
	})

	Context("Charge Order With Referral", func() {
		It("Should charge existing order with referral successfully", func() {
			db := datastore.New(org.Namespaced(ctx))

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

			w := client.PostRawJSON("/order/"+ord1.Id()+"/charge", requests.ValidUserPaymentOnly)
			Expect(w.Code).To(Equal(200))
			log.Debug("JSON %v", w.Body)

			refl := referral.New(db)
			ok, _ := refl.Query().Filter("ReferrerId=", refIn.Id()).First()
			Expect(ok).To(Equal(true))

			trans := transaction.New(db)
			ok, _ = trans.Query().Filter("SourceId=", refIn.Id()).First()
			Expect(ok).To(Equal(true))
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

	Context("Refund Order", func() {
		It("Should refund order successfully", func() {
			db := datastore.New(org.Namespaced(ctx))

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

			w := client.PostRawJSON("/order/"+ordId+"/charge", requests.ValidUserPaymentOnly)
			Expect(w.Code).To(Equal(200))
			log.Debug("JSON %v", w.Body)

			w = client.PostRawJSON("/order/"+ordId+"/refund", requests.NegativeRefund)
			Expect(w.Code).ToNot(Equal(200))

			w = client.PostRawJSON("/order/"+ordId+"/refund", requests.LargeRefundAmount)
			Expect(w.Code).ToNot(Equal(200))

			w = client.PostRawJSON("/order/"+ordId+"/refund", requests.PartialRefund)
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

// Setup appengine context
var _ = BeforeSuite(func() {
	publishedRequired := middleware.TokenRequired(permission.Admin | permission.Published)

	var err error
	ctx, inst, err = ae.NewContext(ae.Options{
		Modules:    []string{"default"},
		TaskQueues: []string{"default"},
	})
	Expect(err).NotTo(HaveOccurred())

	// Mock gin context that we can use with fixtures
	c := gincontext.New(ctx)
	u = fixtures.User(c).(*user.User)
	org = fixtures.Organization(c).(*organization.Organization)
	refIn = fixtures.Referrer(c).(*referrer.Referrer)
	prod = fixtures.Product(c).(*product.Product)
	fixtures.Coupon(c)
	fixtures.Variant(c)
	stor = fixtures.Store(c).(*store.Store)

	nsDb := datastore.New(org.Namespaced(ctx))
	ap = app.New(nsDb)
	ap.GetById(organization.DefaultAppName)

	// Setup client and add routes for payment API tests.
	client = ginclient.New(ctx)
	checkoutApi.Route(client.Router, publishedRequired)
	orderApi.Route(client.Router, publishedRequired)
	storeApi.Route(client.Router, publishedRequired)

	// Create organization for tests, apiKey
	testTok, _, err := ap.GetApiKeyByName(app.TestSecretKey)
	Expect(err).NotTo(HaveOccurred())
	apiKey = testTok.String

	// Set authorization header for subsequent requests
	client.Setup(func(r *http.Request) {
		r.Header.Set("Authorization", apiKey)
	})
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	inst.Close()
})
