package test

import (
	"testing"

	"hanzo.io/datastore"
	"hanzo.io/models/media"
	"hanzo.io/models/media/util"
	"hanzo.io/util/test/ae"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("models/tokens", t)
}

var (
	ctx ae.Context
	db  *datastore.Datastore
)

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	db = datastore.New(ctx)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("models/media", func() {
	Context("media.New", func() {
		It("Should use defaults", func() {
			m := media.New(db)
			Expect(m.Type).To(Equal(media.ImageType))
			Expect(m.Usage).To(Equal(media.UnknownUsage))
			Expect(m.IsParent).To(Equal(false))
		})
	})

	Context("media.Fork", func() {
		It("Should fork correctly", func() {
			m := media.New(db)
			m2 := m.Fork()

			Expect(m2.ParentMediaId).To(Equal(m.Id()))
		})
	})

	Context("media.DetermineUsage", func() {
		It("Should work with unknown", func() {
			m := media.New(db)
			m.AdId = ""
			m.ProductId = ""
			u := m.DetermineUsage()
			Expect(m.Usage).To(Equal(u))
			Expect(m.Usage).To(Equal(media.UnknownUsage))
		})

		It("Should work with Ads", func() {
			m := media.New(db)
			m.AdId = "Something"
			m.ProductId = ""
			u := m.DetermineUsage()
			Expect(m.Usage).To(Equal(u))
			Expect(m.Usage).To(Equal(media.AdUsage))
		})

		It("Should work with Products", func() {
			m := media.New(db)
			m.AdId = ""
			m.ProductId = "Something"
			u := m.DetermineUsage()
			Expect(m.Usage).To(Equal(u))
			Expect(m.Usage).To(Equal(media.ProductUsage))
		})
	})

	Context("media.LoaderSaver", func() {
		It("Should save correct IsParent and Usage", func() {
			m := media.New(db)
			m.AdId = ""
			m.ProductId = "Something"
			m.ParentMediaId = "Something"

			Expect(m.Usage).ToNot(Equal(media.ProductUsage))
			Expect(m.IsParent).To(Equal(false))

			m.MustCreate()

			m2 := media.New(db)
			m2.GetById(m.Id())

			Expect(m2.Usage).To(Equal(media.ProductUsage))
			Expect(m2.IsParent).To(Equal(true))
		})
	})

	Context("util.GetParentMedia", func() {
		It("Should work correctly", func() {
			p := media.New(db)
			p.MustCreate()

			m := media.New(db)
			m.ParentMediaId = p.Id()

			p2, err := util.GetParentMedia(db, m)
			Expect(err).ToNot(HaveOccurred())
			Expect(p2.Id()).To(Equal(p.Id()))
		})
	})
})
