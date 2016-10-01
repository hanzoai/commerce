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
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/gincontext"
	"crowdstart.com/util/hashid"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/test/ae"
	"crowdstart.com/util/test/ginclient"

	checkoutApi "crowdstart.com/api/checkout"
	couponApi "crowdstart.com/api/coupon"
	orderApi "crowdstart.com/api/order"
	storeApi "crowdstart.com/api/store"

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

var _ = Describe("GET checkout/authorize", func() {
	Context("Authorize First Time Customers", func() {
		It("Should normalise the user information", func() {
		})

		It("Should authorize new order successfully", func() {
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

	Context("Authorize Returning Customers", func() {
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

		It("Should not save customer with invalid user id for store", func() {
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

	// Other things that could be tested
	// Capturing an unauthorized order
	// Capturing a captured order
	// Authorizing a captured order
})
