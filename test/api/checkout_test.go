package test

import (
	"crowdstart.com/api/checkout"
	"crowdstart.com/models/lineitem"
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/product"
	"crowdstart.com/models/user"
	"crowdstart.com/models/variant"
	"crowdstart.com/util/hashid"
	"crowdstart.com/util/log"

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
	Describe("checkout/authorize", func() {
		var req *checkout.AuthorizationReq
		var res *order.Order

		Before(func() {
			// Create new authorization request
			req = new(checkout.AuthorizationReq)

			// Create fake payment
			req.Payment_ = payment.Fake(db)

			// Create fake user
			req.User_ = user.Fake(db)

			// Create fake product, variant and subsequent order
			p := product.Fake(db)
			p.MustCreate()
			v := variant.Fake(db, p.Id())
			v.MustCreate()
			li := lineitem.Fake(v.Id(), v.Name, v.SKU)
			req.Order = order.Fake(db, li)

			// Instantiate order to encompass result
			res = order.New(db)

			// Make request
			cl.Post("/authorize", req, res)
			log.JSON(res)
		})

		Context("First Time Customers", func() {
			FIt("Should authorize new order successfully", func() {
				getUser(res.UserId)
				// Payment should be in db
				Expect(len(res.PaymentIds)).To(Equal(1))
				getPayment(res.PaymentIds[0])
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
})
