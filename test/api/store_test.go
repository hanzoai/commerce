package test

import (
	"crowdstart.com/models/store"

	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("store", func() {
	Context("New store", func() {
		req := new(store.Store)
		res := new(store.Store)

		Before(func() {
			req = store.Fake(db)
			res = store.New(db)

			cl.Post("/store", req, res)
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
	Context("Get store", func() {
		req := new(store.Store)
		res := new(store.Store)

		Before(func() {
			req = store.Fake(db)
			req.MustCreate()

			res = store.New(db)

			cl.Get("/store/"+req.Id(), res)
		})

		It("Should get stores", func() {
			Expect(res.Name).To(Equal(req.Name))
			Expect(res.Slug).To(Equal(req.Slug))
			Expect(res.Domain).To(Equal(req.Domain))
			Expect(res.Prefix).To(Equal(req.Prefix))
			Expect(res.Currency).To(Equal(req.Currency))
			Expect(res.Address).To(Equal(req.Address))
			Expect(res.Email).To(Equal(req.Email))
		})
	})
	Context("Delete store", func() {
		res := ""

		Before(func() {
			req := store.Fake(db)
			req.MustCreate()

			cl.Delete("/store/" + req.Id())
			res = req.Id()
		})

		It("Should delete stores", func() {
			stor := store.New(db)
			err := stor.GetById(res)
			Expect(err).ToNot(BeNil())
		})
	})
})
