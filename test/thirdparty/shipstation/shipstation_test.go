package test

import (
	"hanzo.io/api/checkout"
	"hanzo.io/models/lineitem"
	"hanzo.io/models/order"
	"hanzo.io/models/payment"
	"hanzo.io/models/product"
	"hanzo.io/models/user"
	"hanzo.io/models/variant"
	"hanzo.io/util/log"

	. "hanzo.io/util/test/ginkgo"
)

var _ = Describe("shipstation", func() {
	Context("Export", func() {
		Before(func() {
			prod := product.Fake(db)
			prod.MustCreate()
			vari := variant.Fake(db, prod.Id())
			vari.MustCreate()
			li := lineitem.Fake(vari)
			ord := order.Fake(db, li)

			req := new(checkout.Authorization)
			req.Order = ord
			req.Payment = payment.Fake(db)
			req.User = user.Fake(db)

			// Create orders
			cl.Post("/checkout/charge", req, nil)
			cl.Post("/checkout/charge", req, nil)
			cl.Post("/checkout/charge", req, nil)
		})

		It("Should export orders", func() {
			w := bacl.Get("/shipstation/suchtees?action=export&start_date=01/02/2006 15:04&end_date=01/01/2020 16:20&page=1", nil)
			log.Error(w.Body)
		})
	})
})
