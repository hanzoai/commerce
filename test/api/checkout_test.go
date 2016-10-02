package test

import (
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/user"
	"crowdstart.com/util/hashid"

	. "crowdstart.com/util/test/ginkgo"
)

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

var _ = Describe("checkout", func() {
	Context("Authorize First Time Customers", func() {
		It("Should normalize the user information", func() {
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
