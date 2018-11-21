package test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/plan"
	"hanzo.io/models/referrer"
	"hanzo.io/models/store"
	"hanzo.io/models/subscription"
	"hanzo.io/models/user"
	"hanzo.io/test/api/subscribe/requests"
	"hanzo.io/thirdparty/stripe"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/json"
	"hanzo.io/util/log"
	"hanzo.io/util/permission"
	"hanzo.io/util/test/ae"
	"hanzo.io/util/test/ginclient"

	subscriptionApi "hanzo.io/api/subscription"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("api/subscribe", t)
}

var (
	ctx         ae.Context
	client      *ginclient.Client
	accessToken string
	db          *datastore.Datastore
	org         *organization.Organization
	pln         *plan.Plan
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
	u = fixtures.User(c).(*user.User)
	org = fixtures.Organization(c).(*organization.Organization)
	refIn = fixtures.Referrer(c).(*referrer.Referrer)
	pln = fixtures.Plan(c).(*plan.Plan)
	fixtures.Coupon(c)
	fixtures.Variant(c)
	stor = fixtures.Store(c).(*store.Store)

	// Setup client and add routes for payment API tests.
	client = ginclient.New(ctx)
	subscriptionApi.Route(client.Router, adminRequired)

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

var _ = Describe("subscription", func() {
	Context("First Time Users Subscribe To Plan", func() {
		It("Should save new subscription successfully", func() {
			path := "/subscribe"
			log.Debug("Path %v", path)

			// Make first request
			w := client.PostRawJSON(path, requests.ValidSubscription)
			Expect(w.Code).To(Equal(200))

			log.Debug("JSON %v", w.Body)

			// Decode body
			sub := subscription.New(db)
			err := json.DecodeBuffer(w.Body, &sub)
			Expect(err).ToNot(HaveOccurred())

			log.Debug("Subscription %v", sub)

			// Fetch the user from the datastore
			usr := user.New(db)
			err = usr.Get(sub.UserId)
			Expect(err).ToNot(HaveOccurred())
			Expect(usr.Key()).ToNot(BeNil())
			stripeVerifyUser(usr)

			// Fetch the plan from the datastore
			pln := plan.New(db)
			err = pln.Get(sub.PlanId)
			Expect(err).ToNot(HaveOccurred())
			Expect(pln.Key()).ToNot(BeNil())
			stripeVerifyPlan(pln)

			stripeVerifyCards(usr, []string{sub.Account.CardId})
		})
	})

	Context("Returning Users Subscribe To Plan", func() {
		It("Should save new subscription successfully", func() {
			path := "/subscribe"
			log.Debug("Path %v", path)

			// Make first request
			w := client.PostRawJSON(path, requests.ValidSubscription)
			Expect(w.Code).To(Equal(200))

			log.Debug("JSON %v", w.Body)

			// Decode body
			sub1 := subscription.New(db)
			err := json.DecodeBuffer(w.Body, &sub1)
			Expect(err).ToNot(HaveOccurred())

			log.Debug("Subscription %v", sub1)

			// Fetch the user from the datastore
			usr1 := user.New(db)
			err = usr1.Get(sub1.UserId)
			Expect(err).ToNot(HaveOccurred())
			Expect(usr1.Key()).ToNot(BeNil())
			stripeVerifyUser(usr1)

			// Fetch the plan from the datastore
			pln := plan.New(db)
			err = pln.Get(sub1.PlanId)
			Expect(err).ToNot(HaveOccurred())
			Expect(pln.Key()).ToNot(BeNil())
			stripeVerifyPlan(pln)

			// Returning user, should reuse stripe customer id
			body := fmt.Sprintf(requests.ReturningSubscription, usr1.Id())
			log.Debug("JSON %v", w.Body)
			w = client.PostRawJSON(path, body)
			Expect(w.Code).To(Equal(200))

			// Decode body
			sub2 := subscription.New(db)
			err = json.DecodeBuffer(w.Body, &sub2)
			Expect(err).ToNot(HaveOccurred())

			log.Debug("Subscription %v", sub2)

			// Fetch the user from the datastore
			usr2 := user.New(db)
			err = usr2.Get(sub2.UserId)
			Expect(err).ToNot(HaveOccurred())
			Expect(usr2.Id()).To(Equal(usr1.Id()))
		})
	})

	Context("Users Update Subscription", func() {
		It("Should update subscription", func() {
			path := "/subscribe"
			log.Debug("Path %v", path)

			// Make first request
			w := client.PostRawJSON(path, requests.ValidSubscription)
			Expect(w.Code).To(Equal(200))

			log.Debug("JSON %v", w.Body)

			// Decode body
			sub1 := subscription.New(db)
			err := json.DecodeBuffer(w.Body, &sub1)
			Expect(err).ToNot(HaveOccurred())

			log.Debug("Subscription %v", sub1)

			// Fetch the user from the datastore
			usr1 := user.New(db)
			err = usr1.Get(sub1.UserId)
			Expect(err).ToNot(HaveOccurred())
			Expect(usr1.Key()).ToNot(BeNil())
			stripeVerifyUser(usr1)

			// Fetch the plan from the datastore
			pln := plan.New(db)
			err = pln.Get(sub1.PlanId)
			Expect(err).ToNot(HaveOccurred())
			Expect(pln.Key()).ToNot(BeNil())
			stripeVerifyPlan(pln)

			// Returning user, should reuse stripe customer id
			w = client.PostRawJSON(path+"/"+sub1.Id(), requests.UpdateSubscription)
			Expect(w.Code).To(Equal(200))

			// Decode body
			sub2 := subscription.New(db)
			err = json.DecodeBuffer(w.Body, &sub2)
			Expect(err).ToNot(HaveOccurred())

			log.Debug("Subscription %v", sub2)

			Expect(sub2.Quantity).To(Equal(2))
		})
	})

	Context("User Unsubscribe To Subscription", func() {
		It("Should unsubscribe successfully", func() {
			path := "/subscribe"
			log.Debug("Path %v", path)

			// Make first request
			w := client.PostRawJSON(path, requests.ValidSubscription)
			Expect(w.Code).To(Equal(200))

			log.Debug("JSON %v", w.Body)

			// Decode body
			sub1 := subscription.New(db)
			err := json.DecodeBuffer(w.Body, &sub1)
			Expect(err).ToNot(HaveOccurred())

			log.Debug("Subscription %v", sub1)

			// Fetch the user from the datastore
			usr1 := user.New(db)
			err = usr1.Get(sub1.UserId)
			Expect(err).ToNot(HaveOccurred())
			Expect(usr1.Key()).ToNot(BeNil())
			stripeVerifyUser(usr1)

			// Fetch the plan from the datastore
			pln := plan.New(db)
			err = pln.Get(sub1.PlanId)
			Expect(err).ToNot(HaveOccurred())
			Expect(pln.Key()).ToNot(BeNil())
			stripeVerifyPlan(pln)

			// Returning user, should reuse stripe customer id
			req, err := http.NewRequest("DELETE", path+"/"+sub1.Id(), nil)
			req.Header.Set("Authorization", accessToken)

			Expect(err).ToNot(HaveOccurred())

			w = client.Do(req)
			responseBytes, err := ioutil.ReadAll(w.Body)
			Expect(err).ToNot(HaveOccurred())

			log.Warn("Response Bytes: %v", string(responseBytes))
			Expect(w.Code).To(Equal(200))

			sub2 := subscription.New(db)
			sub2.GetById(sub1.Id())
			Expect(sub2.EndCancel).To(Equal(true))
			Expect(sub2.CanceledAt).ToNot(BeNil())
			Expect(sub2.Ended).To(Equal(sub2.PeriodEnd))

			// // Decode body
			// sub2 := subscription.New(db)
			// err = json.DecodeBuffer(w.Body, &sub2)
			// Expect(err).ToNot(HaveOccurred())

			// log.Debug("Subscription %v", sub2)

			// // Fetch the user from the datastore
			// usr2 := user.New(db)
			// err = usr2.Get(sub2.UserId)
			// Expect(err).ToNot(HaveOccurred())
			// Expect(usr2.Id()).To(Equal(usr1.Id()))
		})
	})
})
