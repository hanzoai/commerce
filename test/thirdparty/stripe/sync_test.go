package test

import (
	"time"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/thirdparty/stripe"
	"crowdstart.io/thirdparty/stripe/tasks"
	. "crowdstart.io/util/test/ginkgo"
)

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
			charge := models.Charge{
				Captured: true,
				ID:       "ch_15ZHJOCSRlllXCwPWFGgftzK",
				Email:    user.Email,
			}
			key, order := newOrder(user, charge)

			sc := stripe.NewApiClient(ctx, config.Stripe.APISecret)
			tasks.SynchronizeCharge(db, key, *order, sc)

			var updatedCharge models.Charge
			var updatedOrder models.Order
			err := db.Get(key, &updatedOrder)
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
			key, order := newOrder(user, charge)

			sc := stripe.NewApiClient(ctx, config.Stripe.APISecret)
			tasks.SynchronizeCharge(db, key, *order, sc)

			updatedOrder := new(models.Order)
			err := db.Get(key, updatedOrder)
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedOrder.Disputed).To(Equal(true))
		})
	})
})
