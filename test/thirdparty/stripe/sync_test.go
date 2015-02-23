package test

import (
	"testing"
	"time"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/queries"
	"crowdstart.io/util/test/ae"
	. "crowdstart.io/util/test/ginkgo"
)

var (
	ctx      ae.Context
	db       *datastore.Datastore
	q        *queries.Client
	campaign models.Campaign
)

func Test(t *testing.T) {
	Setup("thirdparty/stripe", t)
}

var _ = BeforeSuite(func() {
	// gob.Register(models.Campaign{})
	ctx = ae.NewContext()
	db = datastore.New(ctx)
	q = queries.New(ctx)

	campaign.Id = "dev@hanzo.ai"
	campaign.Creator.Email = campaign.Id
	campaign.Stripe.UserId = "acct_something"
	campaign.Stripe.Livemode = false
	campaign.Stripe.AccessToken = config.Stripe.APISecret
})

var _ = AfterSuite(func() {
	ctx.Close()
})

func newOrder(user *models.User, charge models.Charge) (datastore.Key, *models.Order) {
	key := db.AllocateIntKey("order")

	order := new(models.Order)
	order.Id = key.Encode()
	order.UserId = user.Id
	order.Email = user.Email
	order.CampaignId = campaign.Id
	order.CreatedAt = time.Now()
	order.UpdatedAt = order.CreatedAt
	order.Test = true
	order.Preorder = true
	order.Shipping = 0
	order.Subtotal = 50 * 100
	order.Total = 50 * 100

	charge.Amount = order.Total
	order.Charges = append(order.Charges, charge)
	_, err := db.Put(key, order)
	Expect(err).ToNot(HaveOccurred())

	return key, order
}

func newUser(email string) (datastore.Key, *models.User) {
	user := new(models.User)
	key := db.AllocateIntKey("user")
	user.Id = key.Encode()
	user.Email = email

	_, err := db.Put(key, user)
	Expect(err).ToNot(HaveOccurred())

	return key, user
}

var _ = Describe("SynchronizeCharges", func() {
	Context("Running the task", func() {
		It("Update orders with charges, using information from Stripe", func() {
			_, user := newUser("dev@hanzo.ai")

			var charge models.Charge
			charge.Captured = true
			charge.ID = "ch_15ZHJOCSRlllXCwPWFGgftzK"
			charge.Email = user.Email

			_, order := newOrder(user, charge)

			var updatedCharge models.Charge
			var updatedOrder models.Order
			err := db.GetKind("order", order.Id, &updatedOrder)
			Expect(err).ToNot(HaveOccurred())
			updatedCharge = order.Charges[0]
			Expect(updatedCharge).ToNot(Equal(charge))
		})
	})

	Context("Disputed charge", func() {
		It("should be marked as disputed.", func() {
			_, user := newUser("dev@hanzo.ai")

			charge := models.Charge{
				ID:    "ch_15ZGKCCSRlllXCwPryrymFEH",
				Email: user.Email,
			}

			key, _ := newOrder(user, charge)

			time.Sleep(10 * time.Second)

			updatedOrder := new(models.Order)
			err := db.Get(key, updatedOrder)
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedOrder.Disputed).To(Equal(true))
		})
	})
})
