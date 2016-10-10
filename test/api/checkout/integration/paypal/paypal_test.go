package paypal_test

import (
	"crowdstart.com/api/checkout"
	"crowdstart.com/models/lineitem"
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/product"
	"crowdstart.com/models/store"
	"crowdstart.com/models/user"
	"crowdstart.com/models/variant"
	"crowdstart.com/util/log"

	. "crowdstart.com/util/test/ginkgo"
)

func CancelPaypal(stor *store.Store) {
	// ret := checkoutApi.PayKeyResponse{}
	// cl.Post("/paypal/pay", requests.ValidOrder, &ret, 200)

	// path := "/paypal/cancel/" + ret.PayKey + "?token=" + accessToken
	// if stor != nil {
	// 	path = "/store/" + stor.Id() + path
	// }

	// log.Debug("Path %v", path)

	// // Should come back with 200
	// cl.Post(path, "{}", nil, 200)

	// // Payment should be in db
	// pay := payment.New(db)
	// err := pay.Get(ret.Payments[0].Id())

	// Expect(err).ToNot(HaveOccurred())
	// Expect(string(pay.Status)).To(Equal(string(payment.Cancelled)))

	// // Order should be in db
	// ord := order.New(db)
	// err = ord.Get(pay.OrderId)
	// log.Debug("ord %v", ord)
	// Expect(err).ToNot(HaveOccurred())
	// Expect(ord.Type).To(Equal("paypal"))
	// Expect(string(ord.Status)).To(Equal(string(order.Cancelled)))
	// Expect(ord.FulfillmentStatus).To(Equal(FulfillmentUnfulfilled))
	// Expect(string(ord.PaymentStatus)).To(Equal(string(payment.Cancelled)))

	// // User should be in db
	// usr := user.New(db)
	// err = usr.Get(ord.UserId)

	// Expect(err).ToNot(HaveOccurred())
	// Expect(usr.Key()).ToNot(BeNil())
}

func ConfirmPaypal(stor *store.Store) {
	// ret := checkoutApi.PayKeyResponse{}
	// cl.Post("/paypal/pay", requests.ValidOrder, &ret, 200)

	// path := "/paypal/confirm/" + ret.PayKey + "?token=" + accessToken
	// if stor != nil {
	// 	path = "/store/" + stor.Id() + path
	// }

	// log.Debug("Path %v", path)

	// // Should come back with 200
	// cl.Post(path, "{}", nil)

	// // Payment should be in db
	// pay := payment.New(db)
	// err := pay.Get(ret.Payments[0].Id())

	// Expect(err).ToNot(HaveOccurred())
	// Expect(string(pay.Status)).To(Equal(payment.Paid))

	// // Order should be in db
	// ord := order.New(db)
	// err = ord.Get(pay.OrderId)

	// Expect(err).ToNot(HaveOccurred())
	// Expect(ord.Type).To(Equal("paypal"))
	// Expect(string(ord.Status)).To(Equal(string(order.Open)))
	// Expect(ord.FulfillmentStatus).To(Equal(FulfillmentUnfulfilled))
	// Expect(string(ord.PaymentStatus)).To(Equal(string(payment.Paid)))

	// // User should be in db
	// usr := user.New(db)
	// err = usr.Get(ord.UserId)

	// Expect(err).ToNot(HaveOccurred())
	// Expect(usr.Key()).ToNot(BeNil())
}

func newAuthorization() checkout.AuthorizationReq {
	// Create fake product, variant and subsequent order
	prod := product.Fake(db)
	prod.MustCreate()

	vari := variant.Fake(db, prod.Id())
	vari.MustCreate()

	li := lineitem.Fake(vari)

	usr := user.Fake(db)

	ord := order.Fake(db, li)
	ord.Type = payment.PayPal

	pay := payment.Fake(db)
	pay.Type = payment.PayPal
	pay.Amount = ord.Total

	auth := checkout.AuthorizationReq{usr, pay, ord}
	log.JSON(auth)
	return auth
}

var _ = Describe("payment/paypal", func() {
	Before(func() {

	})

	Context("Get a PayPal PayKey", func() {
		It("Should Get a PayPal PayKey", func() {
			paths := []string{
				"/paypal/pay",
				"/store/" + stor.Id() + "/paypal/pay",
			}

			for _, path := range paths {
				// Should come back with 200
				req := newAuthorization()
				res := order.New(db)
				cl.Post(path, req, res, 200)

				// Payment and Order info should be in the db
				pay := payment.New(db)
				ok, err := pay.Query().Filter("OrderId=", res.Id()).First()
				log.Debug("Err %v", err)

				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				// Order should be in db
				ord := order.New(db)
				err = ord.Get(pay.OrderId)
				Expect(err).ToNot(HaveOccurred())
				log.Debug("Ord %v", ord)
				Expect(string(ord.Type)).To(Equal("paypal"))

				// User should be in db
				usr := user.New(db)
				err = usr.Get(ord.UserId)

				Expect(err).ToNot(HaveOccurred())
				Expect(usr.Key()).ToNot(BeNil())

				log.Debug("Payment: %v", pay)
				log.Debug("Order: %v", ord)
			}
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
