package test

import (
	"net/http"
	"testing"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/fixtures"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/product"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/store"
	"crowdstart.com/models/user"
	"crowdstart.com/test/api/payment/requests"
	"crowdstart.com/thirdparty/paypal"
	"crowdstart.com/util/gincontext"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/test/ae"
	"crowdstart.com/util/test/ginclient"

	orderApi "crowdstart.com/api/order"
	paymentApi "crowdstart.com/api/payment"
	storeApi "crowdstart.com/api/store"

	. "crowdstart.com/util/test/ginkgo"
)

func Test(t *testing.T) {
	log.SetVerbose(testing.Verbose())
	Setup("api/payment/paypal", t)
}

var (
	ctx         ae.Context
	client      *ginclient.Client
	accessToken string
	db          *datastore.Datastore
	org         *organization.Organization
	prod        *product.Product
	stor        *store.Store
	pc          *paypal.Client
	u           *user.User
	refIn       *referrer.Referrer
)

// Setup appengine context
var _ = BeforeSuite(func() {
	adminRequired := middleware.TokenRequired(permission.Admin)

	ctx = ae.NewContext()

	// Mock gin context that we can use with fixtures
	c := gincontext.New(ctx)
	u = fixtures.User(c).(*user.User)
	org = fixtures.Organization(c).(*organization.Organization)
	refIn = fixtures.Referrer(c).(*referrer.Referrer)
	prod = fixtures.Product(c).(*product.Product)
	fixtures.Coupon(c)
	fixtures.Variant(c)
	stor = fixtures.Store(c).(*store.Store)

	// Setup client and add routes for payment API tests.
	client = ginclient.New(ctx)
	paymentApi.Route(client.Router, adminRequired)
	orderApi.Route(client.Router, adminRequired)
	storeApi.Route(client.Router, adminRequired)

	// Create organization for tests, accessToken
	accessToken = org.AddToken("test-published-key", permission.Admin)
	err := org.Put()
	Expect(err).NotTo(HaveOccurred())

	// Set authorization header for subsequent requests
	client.Setup(func(r *http.Request) {
		r.Header.Set("Authorization", accessToken)
	})

	pc = paypal.New(ctx)

	// Save namespaced db
	db = datastore.New(org.Namespace(ctx))
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

type testHelperReturn struct {
	PayKey   string
	Payments []*payment.Payment
	Orders   []*order.Order
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
	payKeyResponse := paymentApi.PayKeyResponse{}

	err := json.DecodeBuffer(w.Body, &payKeyResponse)
	Expect(err).ToNot(HaveOccurred())

	log.Debug("PayKey Response %v", payKeyResponse)

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
	})
})
