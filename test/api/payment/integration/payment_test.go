package test

import (
	"fmt"
	"net/http"
	"testing"

	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models2/fixtures"
	"crowdstart.io/models2/order"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/payment"
	"crowdstart.io/models2/user"
	"crowdstart.io/test/api/payment/requests"
	"crowdstart.io/thirdparty/stripe"
	"crowdstart.io/util/gincontext"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/permission"
	"crowdstart.io/util/test/ae"
	"crowdstart.io/util/test/ginclient"

	orderApi "crowdstart.io/api/order"
	paymentApi "crowdstart.io/api/payment"

	. "crowdstart.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("api/payment", t)
}

var (
	ctx         ae.Context
	client      *ginclient.Client
	accessToken string
	db          *datastore.Datastore
	org         *organization.Organization
	sc          *stripe.Client
)

// Setup appengine context
var _ = BeforeSuite(func() {
	adminRequired := middleware.TokenRequired(permission.Admin)

	ctx = ae.NewContext()

	// Mock gin context that we can use with fixtures
	c := gincontext.New(ctx)
	fixtures.User(c)
	org = fixtures.Organization(c).(*organization.Organization)
	fixtures.Product(c)
	fixtures.Variant(c)

	// Setup client and add routes for payment API tests.
	client = ginclient.New(ctx)
	paymentApi.Route(client.Router, adminRequired)
	orderApi.Route(client.Router, adminRequired)

	// Create organization for tests, accessToken
	accessToken = org.AddToken("test-published-key", permission.Admin)
	err := org.Put()
	Expect(err).NotTo(HaveOccurred())

	// Set authorization header for subsequent requests
	client.Setup(func(r *http.Request) {
		r.Header.Set("Authorization", accessToken)
	})

	sc = stripe.New(ctx, org.Stripe.Test.AccessToken)

	// Save namespaced db
	db = datastore.New(org.Namespace(ctx))
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

type testHelperReturn struct {
	Payments []*payment.Payment
	Orders   []*order.Order
}

func FirstTimeSuccessfulOrderTest(isCharge bool) testHelperReturn {
	var path string
	if isCharge {
		path = "/charge"
	} else {
		path = "/authorize"
	}

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
	key, err := order.New(db).KeyExists(ord.Id())
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
	var payments []payment.Payment
	payment.Query(db).GetAll(&payments)

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

func ReturningSuccessfulOrderSameCardTest(isCharge bool) testHelperReturn {
	var path string
	if isCharge {
		path = "/charge"
	} else {
		path = "/authorize"
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

func ReturningSuccessfulOrderNewCardTest(isCharge bool) testHelperReturn {
	var path string
	if isCharge {
		path = "/charge"
	} else {
		path = "/authorize"
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

func OrderBadCardTest(isCharge bool) {
	var path string
	if isCharge {
		path = "/charge"
	} else {
		path = "/authorize"
	}

	// Returning user, should reuse stripe customer id
	body := fmt.Sprintf(requests.InvalidOrderBadCard)
	w := client.PostRawJSON(path, body)
	log.Debug("JSON %v", w.Body)
	Expect(w.Code).To(Equal(500))
}

func OrderBadUserTest(isCharge bool) {
	var path string
	if isCharge {
		path = "/charge"
	} else {
		path = "/authorize"
	}

	// Returning user, should reuse stripe customer id
	body := fmt.Sprintf(requests.ReturningUserOrderNewCard, "BadId")
	w := client.PostRawJSON(path, body)
	log.Debug("JSON %v", w.Body)
	Expect(w.Code).To(Equal(500))
}

var _ = Describe("payment", func() {
	Context("Authorize First Time Customers", func() {
		It("Should save new order successfully", func() {
			FirstTimeSuccessfulOrderTest(false)
		})

		It("Should not authorize invalid credit card number", func() {
			OrderBadCardTest(false)
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
			ReturningSuccessfulOrderSameCardTest(false)
		})

		It("Should save returning customer order with a new card successfully", func() {
			ReturningSuccessfulOrderNewCardTest(false)
		})

		It("Should not save customer with invalid user id", func() {
			OrderBadUserTest(false)
		})
	})

	Context("Charge First Time Customers", func() {
		It("Should save new order successfully", func() {
			FirstTimeSuccessfulOrderTest(true)
		})

		It("Should not authorize invalid credit card number", func() {
			OrderBadCardTest(true)
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
			ReturningSuccessfulOrderSameCardTest(true)
		})

		It("Should save returning customer order with a new card successfully", func() {
			ReturningSuccessfulOrderNewCardTest(true)
		})

		It("Should not save customer with invalid user id", func() {
			OrderBadUserTest(true)
		})
	})

	Context("Authorize Order", func() {
		It("Should authorize existing order successfully", func() {
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
			Expect(w.Code).To(Equal(500))
			log.Debug("JSON %v", w.Body)
		})
	})

	Context("Capture Order", func() {
		It("Should capture existing authorized order successfully", func() {
			pnos := FirstTimeSuccessfulOrderTest(false)
			id := pnos.Orders[0].Id()

			w := client.PostRawJSON("/order/"+id+"/capture", "")
			Expect(w.Code).To(Equal(200))
			log.Debug("JSON %v", w.Body)
			stripeVerifyCharge(pnos.Payments[0])
		})

		It("Should not capture invalid order", func() {
			w := client.PostRawJSON("/order/BADID/capture", "")
			Expect(w.Code).To(Equal(500))
			log.Debug("JSON %v", w.Body)
		})
	})

	Context("Charge Order", func() {
		It("Should charge existing order successfully", func() {
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
			Expect(w.Code).To(Equal(500))
			log.Debug("JSON %v", w.Body)
		})
	})

	// Other things that could be tested
	// Capturing an unauthorized order
	// Capturing a captured order
	// Authorizing a captured order
})
