package test

import (
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/store"
	"crowdstart.com/models/user"
	"crowdstart.com/test/api/checkout/integration/requests"
	"crowdstart.com/util/log"

	checkoutApi "crowdstart.com/api/checkout"

	. "crowdstart.com/models"
	. "crowdstart.com/util/test/ginkgo"
)

type testHelperReturn struct {
	PayKey   string
	Payments []*payment.Payment
	Orders   []*order.Order
}

func CancelPaypal(stor *store.Store) testHelperReturn {
	ret := GetPayKey(stor)

	path := "/paypal/cancel/" + ret.PayKey + "?token=" + accessToken
	if stor != nil {
		path = "/store/" + stor.Id() + path
	}

	log.Debug("Path %v", path)

	// Should come back with 200
	cl.Post(path, "{}", nil, 200)

	// Payment should be in db
	pay := payment.New(db)
	err := pay.Get(ret.Payments[0].Id())

	Expect(err).ToNot(HaveOccurred())
	Expect(string(pay.Status)).To(Equal(string(payment.Cancelled)))

	// Order should be in db
	ord := order.New(db)
	err = ord.Get(pay.OrderId)
	log.Debug("ord %v", ord)
	Expect(err).ToNot(HaveOccurred())
	Expect(ord.Type).To(Equal("paypal"))
	Expect(string(ord.Status)).To(Equal(string(order.Cancelled)))
	Expect(ord.FulfillmentStatus).To(Equal(FulfillmentUnfulfilled))
	Expect(string(ord.PaymentStatus)).To(Equal(string(payment.Cancelled)))

	// User should be in db
	usr := user.New(db)
	err = usr.Get(ord.UserId)

	Expect(err).ToNot(HaveOccurred())
	Expect(usr.Key()).ToNot(BeNil())

	return ret
}

func ConfirmPaypal(stor *store.Store) testHelperReturn {
	ret := GetPayKey(stor)

	path := "/paypal/confirm/" + ret.PayKey + "?token=" + accessToken
	if stor != nil {
		path = "/store/" + stor.Id() + path
	}

	log.Debug("Path %v", path)

	// Should come back with 200
	cl.Post(path, "{}", nil)

	// Payment should be in db
	pay := payment.New(db)
	err := pay.Get(ret.Payments[0].Id())

	Expect(err).ToNot(HaveOccurred())
	Expect(string(pay.Status)).To(Equal(payment.Paid))

	// Order should be in db
	ord := order.New(db)
	err = ord.Get(pay.OrderId)

	Expect(err).ToNot(HaveOccurred())
	Expect(ord.Type).To(Equal("paypal"))
	Expect(string(ord.Status)).To(Equal(string(order.Open)))
	Expect(ord.FulfillmentStatus).To(Equal(FulfillmentUnfulfilled))
	Expect(string(ord.PaymentStatus)).To(Equal(string(payment.Paid)))

	// User should be in db
	usr := user.New(db)
	err = usr.Get(ord.UserId)

	Expect(err).ToNot(HaveOccurred())
	Expect(usr.Key()).ToNot(BeNil())

	return ret
}

func GetPayKey(stor *store.Store) testHelperReturn {
	path := "/paypal/pay"
	if stor != nil {
		path = "/store/" + stor.Id() + path
	}

	// Should come back with 200
	payKeyResponse := checkoutApi.PayKeyResponse{}
	cl.Post(path, requests.ValidOrder, &payKeyResponse, 200)

	// Payment and Order info should be in the db
	pay := payment.New(db)
	ok, err := pay.Query().Filter("Account.PayKey=", payKeyResponse.PayKey).First()
	log.Debug("Err %v", err)

	Expect(err).ToNot(HaveOccurred())
	Expect(ok).To(BeTrue())

	// Order should be in db
	ord := order.New(db)
	err = ord.Get(pay.OrderId)
	Expect(err).ToNot(HaveOccurred())
	log.Debug("Ord %v", ord)
	Expect(ord.Type).To(Equal("paypal"))

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

		It("Should Get a PayPal PayKey For Store", func() {
			log.Debug("Results: %v", GetPayKey(stor))
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
