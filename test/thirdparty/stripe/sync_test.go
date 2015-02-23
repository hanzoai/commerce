package test

import (
	"encoding/gob"
	"path/filepath"
	"testing"
	"time"

	"github.com/davidtai/appenginetesting"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/jmcvetta/randutil"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
	"crowdstart.io/thirdparty/stripe"
	"crowdstart.io/util/log"
	"crowdstart.io/util/queries"
)

var (
	ctx      *appenginetesting.Context
	db       *datastore.Datastore
	q        *queries.Client
	campaign models.Campaign
)

func TestStripeSync(t *testing.T) {
	var err error
	ctx, err = appenginetesting.NewContext(&appenginetesting.Options{
		AppId:      "crowdstart-io",
		Debug:      appenginetesting.LogWarning,
		Testing:    t,
		TaskQueues: []string{"default"},
		Modules: []appenginetesting.ModuleConfig{
			{
				Name: "default",
				Path: filepath.Join("../../../config/development/app.yaml"),
			},
		},
	})
	if err != nil {
		panic(err)
	}
	time.Sleep(1 * time.Second)

	log.SetVerbose(testing.Verbose())
	RegisterFailHandler(Fail)
	RunSpecs(t, "stripe-sync")
}

var _ = BeforeSuite(func() {
	gob.Register(models.Campaign{})

	db = datastore.New(ctx)
	q = queries.New(ctx)

	campaign.Id = "dev@hanzo.ai"
	campaign.Creator.Email = campaign.Id
	campaign.Stripe.UserId = "acct_something"
	campaign.Stripe.Livemode = false
	campaign.Stripe.AccessToken = "sk_test_oGcTBghcS1NvO1XSA3d9FLIP"
})

var _ = AfterSuite(func() {
	ctx.Close()
})

func New(user models.User, charge models.Charge) (models.Order, error) {
	var order models.Order
	order.Id, _ = String(6, Alphanumeric)
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
	_, err := db.PutKind("order", order.Id, &order)
	Expect(err).ToNot(HaveOccurred())

	return order, parallel.Run(ctx, "order", 2, stripe.SynchronizeCharges, campaign)
}

var _ = Describe("SynchronizeCharges", func() {
	var (
		order  models.Order
		charge models.Charge
	)
	Context("Running the task", func() {
		It("should not error", func() {
			var user models.User
			user.Id, _ = String(6, Alphanumeric)
			user.Email = "test@test.com"
			db.PutKind("user", user.Id, &user)

			charge.Captured = true
			charge.ID = "ch_15XOuYEIkPffEth5yhRqlUay"
			charge.Email = user.Email

			var err error
			order, err = New(user, charge)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("The charge", func() {
		It("should be different", func() {
			var updatedCharge models.Charge
			var updatedOrder models.Order
			err := db.GetKind("order", order.Id, &updatedOrder)
			Expect(err).ToNot(HaveOccurred())
			updatedCharge = order.Charges[0]
			Expect(updatedCharge).ToNot(Equal(charge))
		})
	})

	Context("Disputed charge", func() {
		It("should be marked", func() {
			id, _ := String(6, Alphanumeric)
			user := models.User{
				Email: "test2@test.com",
				Id:    id,
			}
			_, err := db.PutKind("user", user.Id, &user)
			Expect(err).ToNot(HaveOccurred())

			charge := models.Charge{
				ID:    "ch_15XcVJEIkPffEth5eRV81jW0",
				Email: user.Email,
			}
			order, err := New(user, charge)
			Expect(err).ToNot(HaveOccurred())

			time.Sleep(10 * time.Second)

			var updatedOrder models.Order
			err = db.GetKind("order", order.Id, &updatedOrder)
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedOrder.Disputed).To(Equal(true))
		})
	})
})
