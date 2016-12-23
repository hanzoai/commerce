package test

import (
	"crowdstart.com/models/lineitem"
	"crowdstart.com/models/order"
	"crowdstart.com/models/product"
	"crowdstart.com/models/variant"
	"crowdstart.com/util/log"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("shipstation", func() {
	Context("Export", func() {
		Before(func() {
			prod := product.Fake(db)
			prod.MustCreate()
			vari := variant.Fake(db, prod.Id())
			vari.MustCreate()
			li := lineitem.Fake(vari)
			req := order.Fake(db, li)

			// Create orders
			cl.Post("/order", req, nil)
			cl.Post("/order", req, nil)
			cl.Post("/order", req, nil)
		})

		It("Should export orders", func() {
			w := bacl.Get("/shipstation/suchtees?action=export", nil)
			log.Error(w.Body)
		})
	})
})
