package test

import . "crowdstart.com/util/test/ginkgo"

var _ = Describe("SynchronizeCharge", func() {
	Context("Running the task", func() {
		It("Update orders with charges, using information from Stripe", func() {
			// _, user := newUser("dev@hanzo.ai")
			// charge := models.Charge{
			// 	Captured: true,
			// 	ID:       "ch_15ZHJOCSRlllXCwPWFGgftzK",
			// 	Email:    user.Email,
			// }
			// key, order := newOrder(user, charge)

			// sc := stripe.NewApiClient(ctx, config.Stripe.APISecret)
			// tasks.SynchronizeCharge(db, key, *order, sc)

			// var updatedCharge models.Charge
			// var updatedOrder models.Order
			// err := db.Get(key, &updatedOrder)
			// Expect(err).ToNot(HaveOccurred())
			// updatedCharge = order.Charges[0]
			// Expect(updatedCharge).ToNot(Equal(charge))
		})
	})

	// 	Context("Disputed charge", func() {
	// 		It("should be marked as disputed.", func() {
	// 			_, user := newUser("dev@hanzo.ai")
	// 			charge := models.Charge{
	// 				ID:    "ch_15ZGKCCSRlllXCwPryrymFEH",
	// 				Email: user.Email,
	// 			}
	// 			key, order := newOrder(user, charge)

	// 			sc := stripe.NewApiClient(ctx, config.Stripe.APISecret)
	// 			tasks.SynchronizeCharge(db, key, *order, sc)

	// 			updatedOrder := new(models.Order)
	// 			err := db.Get(key, updatedOrder)
	// 			Expect(err).ToNot(HaveOccurred())
	// 			Expect(updatedOrder.Disputed).To(Equal(true))
	// 		})
	// 	})
})
