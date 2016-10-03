package test

import (
	"crowdstart.com/models/store"
	"crowdstart.com/util/log"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("store", func() {
	Context("New store", func() {
		var req *store.Store
		var res *store.Store

		Before(func() {
			req = store.Fake(db)
			res = store.New(db)

			// Create new store
			log.JSON(req)
			cl.Post("/store", req, res)
			log.JSON(res)
		})

		It("Should create new stores", func() {
			Expect(res.Name).To(Equal(req.Name))
			Expect(res.Slug).To(Equal(req.Slug))
			Expect(res.Domain).To(Equal(req.Domain))
			Expect(res.Prefix).To(Equal(req.Prefix))
			Expect(res.Currency).To(Equal(req.Currency))
			Expect(res.Address).To(Equal(req.Address))
			Expect(res.Email).To(Equal(req.Email))
		})
	})
})
