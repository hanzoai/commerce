package test

import (
	"net/http"
	"testing"

	"appengine"

	apiPayment "crowdstart.io/api/payment"
	"crowdstart.io/datastore"
	"crowdstart.io/models2/fixtures"
	"crowdstart.io/models2/order"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/payment"
	"crowdstart.io/models2/user"
	"crowdstart.io/test/api/payment/requests"
	"crowdstart.io/util/gincontext"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/permission"
	"crowdstart.io/util/test/ae"
	"crowdstart.io/util/test/ginclient"

	. "crowdstart.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("api/payment", t)
}

var (
	aectx       ae.Context
	ctx         appengine.Context
	client      *ginclient.Client
	accessToken string
	db          datastore.Datastore
	org         *organization.Organization
)

// Setup appengine context
var _ = BeforeSuite(func() {
	aectx = ae.NewContext()

	// Mock gin context that we can use with fixtures
	c := gincontext.New(aectx)
	fixtures.User(c)
	org = fixtures.Organization(c)
	fixtures.Product(c)
	fixtures.Variant(c)

	// Setup client and add routes for payment API
	client = ginclient.New(aectx)
	apiPayment.Route(client.Router)

	// Create organization for tests, accessToken
	accessToken = org.AddToken("test-published-key", permission.Admin)
	err := org.Put()
	Expect(err).NotTo(HaveOccurred())

	// Set authorization header for subsequent requests
	client.Setup(func(r *http.Request) {
		r.Header.Set("Authorization", accessToken)
	})

	ctx = org.Namespace(aectx)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	aectx.Close()
})

var _ = Describe("Payment API", func() {
	Context("Authorize", func() {
		It("Should save new order successfully", func() {
			// Should come back with 200
			w := client.PostRawJSON("/authorize", requests.ValidOrder)
			Expect(w.Code).To(Equal(200))

			log.Debug("JSON %v", w.Body)

			// Payment and Order info should be in the dv
			db := datastore.New(ctx)
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
			key, err = user.New(db).KeyExists(ord.UserId)

			Expect(err).ToNot(HaveOccurred())
			Expect(key).ToNot(BeNil())

			// Payment should be in db
			Expect(len(ord.PaymentIds)).To(Equal(1))
			var payments []payment.Payment
			payment.Query(db).GetAll(&payments)

			log.Warn("Payments %v", payments)
			key, err = payment.New(db).KeyExists(ord.PaymentIds[0])

			Expect(err).ToNot(HaveOccurred())
			Expect(key).ToNot(BeNil())
		})
	})
})
