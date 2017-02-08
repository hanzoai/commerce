package paypal_integration

import (
	"net/http"
	"testing"

	"golang.org/x/net/context"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/app"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/product"
	"hanzo.io/models/referrer"
	"hanzo.io/models/store"
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

	. "hanzo.io/models"
	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("api/checkout/paypal", t)
}

var (
	ctx    context.Context
	inst   ae.Instance
	client *ginclient.Client
	ap     *app.App
	apiKey string
	db     *datastore.Datastore
	org    *organization.Organization
	prod   *product.Product
	refIn  *referrer.Referrer
	stor   *store.Store
	u      *user.User
)

// Setup appengine context
var _ = BeforeSuite(func() {
	publishedRequired := middleware.TokenRequired(permission.Admin | permission.Published)

	ctx, inst, _ = ae.NewContext()

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
	testTok, _, err := ap.GetApiKeyByName(app.TestPublishedKey)
	Expect(err).NotTo(HaveOccurred())
	apiKey = testTok.String

	// Set authorization header for subsequent requests
	client.Setup(func(r *http.Request) {
		r.Header.Set("Authorization", apiKey)
	})

	// Save namespaced db
	db = datastore.New(org.Namespaced(ctx))
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	inst.Close()
})

type testHelperReturn struct {
	PayKey   string
	Payments []*payment.Payment
	Orders   []*order.Order
}

func CancelPaypal(stor *store.Store) testHelperReturn {
	ret := GetPayKey(stor)

	path := "/paypal/cancel/" + ret.PayKey + "?token=" + apiKey
	if stor != nil {
		path = "/store/" + stor.Id() + path
	}

	log.Debug("Path %v", path)

	// Should come back with 200
	w := client.PostRawJSON(path, "{}")
	Expect(w.Code).To(Equal(200))

	log.Debug("JSON %v", w.Body)

	// Payment should be in db
	pay := payment.New(db)
	err := pay.Get(ret.Payments[0].Id())

	Expect(err).ToNot(HaveOccurred())
	Expect(string(pay.Status)).To(Equal(string(payment.Cancelled)))

	// Order should be in db
	ord := order.New(db)
	err = ord.Get(pay.OrderId)
	log.Debug("ord %v", ord)
	Expect(err).ToNot(HaveOccurred())
	Expect(ord.Type).To(Equal("paypal"))
	Expect(string(ord.Status)).To(Equal(string(order.Cancelled)))
	Expect(ord.FulfillmentStatus).To(Equal(FulfillmentUnfulfilled))
	Expect(string(ord.PaymentStatus)).To(Equal(string(payment.Cancelled)))

	// User should be in db
	usr := user.New(db)
	err = usr.Get(ord.UserId)

	Expect(err).ToNot(HaveOccurred())
	Expect(usr.Key()).ToNot(BeNil())

	return ret
}

func ConfirmPaypal(stor *store.Store) testHelperReturn {
	ret := GetPayKey(stor)

	path := "/paypal/confirm/" + ret.PayKey + "?token=" + apiKey
	if stor != nil {
		path = "/store/" + stor.Id() + path
	}

	log.Debug("Path %v", path)

	// Should come back with 200
	w := client.PostRawJSON(path, "{}")
	Expect(w.Code).To(Equal(200))

	log.Debug("JSON %v", w.Body)

	// Payment should be in db
	pay := payment.New(db)
	err := pay.Get(ret.Payments[0].Id())

	Expect(err).ToNot(HaveOccurred())
	Expect(string(pay.Status)).To(Equal(payment.Paid))

	// Order should be in db
	ord := order.New(db)
	err = ord.Get(pay.OrderId)

	Expect(err).ToNot(HaveOccurred())
	Expect(ord.Type).To(Equal("paypal"))
	Expect(string(ord.Status)).To(Equal(string(order.Open)))
	Expect(ord.FulfillmentStatus).To(Equal(FulfillmentUnfulfilled))
	Expect(string(ord.PaymentStatus)).To(Equal(string(payment.Paid)))

	// User should be in db
	usr := user.New(db)
	err = usr.Get(ord.UserId)

	Expect(err).ToNot(HaveOccurred())
	Expect(usr.Key()).ToNot(BeNil())

	return ret
}

func GetPayKey(stor *store.Store) testHelperReturn {
	path := "/paypal/pay"
	if stor != nil {
		path = "/store/" + stor.Id() + path
	}

	// Should come back with 200
	w := client.PostRawJSON(path, requests.ValidOrder)
	Expect(w.Code).To(Equal(200))

	log.Debug("JSON %v", w.Body)

	// Payment and Order info should be in the db
	payKeyResponse := checkoutApi.PayKeyResponse{}

	err := json.DecodeBuffer(w.Body, &payKeyResponse)
	Expect(err).ToNot(HaveOccurred())

	log.Debug("PayKey Response %v", payKeyResponse.PayKey)

	// Payment should be in db
	pay := payment.New(db)
	ok, err := pay.Query().Filter("Account.PayKey=", payKeyResponse.PayKey).First()
	log.Debug("Err %v", err)

	Expect(err).ToNot(HaveOccurred())
	Expect(ok).To(BeTrue())

	// Order should be in db
	ord := order.New(db)
	err = ord.Get(pay.OrderId)
	Expect(err).ToNot(HaveOccurred())
	log.Debug("Ord %v", ord)
	Expect(ord.Type).To(Equal("paypal"))

	// User should be in db
	usr := user.New(db)
	err = usr.Get(ord.UserId)

	Expect(err).ToNot(HaveOccurred())
	Expect(usr.Key()).ToNot(BeNil())

	return testHelperReturn{
		PayKey:   payKeyResponse.PayKey,
		Payments: []*payment.Payment{pay},
		Orders:   []*order.Order{ord},
	}
}

var _ = Describe("payment/paypal", func() {
	Context("Get a PayPal PayKey", func() {
		It("Should Get a PayPal PayKey", func() {
			log.Debug("Results: %v", GetPayKey(nil))
		})

		It("Should Get a PayPal PayKey For Store", func() {
			log.Debug("Results: %v", GetPayKey(stor))
		})
	})

	// Context("Finish a PayPal Order", func() {
	// 	It("Should Complete an Order", func() {
	// 		log.Debug("Results: %v", ConfirmPaypal(nil))
	// 	})

	// 	It("Should Complete an Order For Store", func() {
	// 		log.Debug("Results: %v", ConfirmPaypal(stor))
	// 	})

	// 	It("Should Cancel an Order", func() {
	// 		log.Debug("Results: %v", CancelPaypal(nil))
	// 	})

	// 	It("Should Cancel an Order For Store", func() {
	// 		log.Debug("Results: %v", CancelPaypal(stor))
	// 	})
	// })
})
