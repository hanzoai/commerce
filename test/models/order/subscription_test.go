package test

import (
	"hanzo.io/models/order"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/rand"

	. "hanzo.io/util/test/ginkgo"
)

var _ = Describe("Order.Subscription", func() {
	FContext("CreateSubscriptionsFromItems", func() {
		BeforeEach(func() {
			// Scramble currency values so we know they are being replaced
			for i, _ := range ord.Coupons {
				ord.Coupons[i].Amount = rand.Int()
			}

			for i, _ := range ord.Items {
				ord.Items[i].Price = currency.Cents(rand.Int64())
			}

			ord.LineTotal = currency.Cents(rand.Int64())
			ord.Discount = currency.Cents(rand.Int64())
			ord.Subtotal = currency.Cents(rand.Int64())
			ord.Tax = 0      //currency.Cents(rand.Int64())
			ord.Shipping = 0 //currency.Cents(rand.Int64())
			ord.Total = currency.Cents(rand.Int64())
			ord.TokenSaleId = ""
			ord.WalletId = ""
			ord.WalletPassphrase = ""
			ord.Mode = order.DefaultMode

			ord.Subscriptions = make([]order.Subscription, 0)
		})

		FIt("Should Create Subscriptions From Items", func () {
			err := ord.CreateSubscriptionsFromItems(stor)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(ord.Subscriptions)).To(Equal(1))
			sub := ord.Subscriptions[0]
			Expect(sub.Status).To(Equal(order.UnpaidSubscriptionStatus))
			Expect(sub.ProductCachedValues).To(Equal(subProd.ProductCachedValues))

			Expect(sub.Price).To(Equal(subProd.Price))

			tax := 1 + currency.Cents(float64(sub.Price)*0.0885)
			shipping := 499 + currency.Cents(float64(sub.Price)*0.1)

			Expect(sub.Tax).To(Equal(tax))
			Expect(sub.Shipping).To(Equal(shipping))
			Expect(sub.Total).To(Equal(sub.Price + tax + shipping))
		})

		It("Should Create Multiple Subscriptions From Item Quantities", func () {
			ord.Items[2].Quantity = 2
			err := ord.CreateSubscriptionsFromItems(stor)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(ord.Subscriptions)).To(Equal(2))
			sub := ord.Subscriptions[0]
			Expect(sub.Status).To(Equal(order.UnpaidSubscriptionStatus))
			Expect(sub.ProductCachedValues).To(Equal(subProd.ProductCachedValues))

			sub = ord.Subscriptions[1]
			Expect(sub.Status).To(Equal(order.UnpaidSubscriptionStatus))
			Expect(sub.ProductCachedValues).To(Equal(subProd.ProductCachedValues))
			ord.Items[2].Quantity = 1
		})
	})
})
