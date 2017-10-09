package test

import (
	"testing"

	"hanzo.io/datastore"
	"hanzo.io/models/copy"
	"hanzo.io/models/copy/util"
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

var _ = Describe("models/copy", func() {
	Context("copy.New", func() {
		It("Should use defaults", func() {
			m := copy.New(db)
			Expect(m.Type).To(Equal(copy.ContentType))
			Expect(m.IsParent).To(Equal(false))
		})
	})

	Context("copy.Fork", func() {
		It("Should fork correctly", func() {
			m := copy.New(db)
			m2 := m.Fork()

			Expect(m2.ParentCopyId).To(Equal(m.Id()))
		})
	})

	Context("copy.LoaderSaver", func() {
		It("Should save correct IsParent and Usage", func() {
			m := copy.New(db)
			m.AdId = ""
			m.ParentCopyId = "Something"

			Expect(m.IsParent).To(Equal(false))

			m.MustCreate()

			m2 := copy.New(db)
			m2.GetById(m.Id())

			Expect(m2.IsParent).To(Equal(true))
		})
	})

	Context("util.GetParentCopy", func() {
		It("Should work correctly", func() {
			p := copy.New(db)
			p.MustCreate()

			m := copy.New(db)
			m.ParentCopyId = p.Id()

			p2, err := util.GetParentCopy(db, m)
			Expect(err).ToNot(HaveOccurred())
			Expect(p2.Id()).To(Equal(p.Id()))
		})

		It("Should error correctly", func() {
			m := copy.New(db)

			_, err := util.GetParentCopy(db, m)
			Expect(err).To(Equal(util.NoParentCopyFound))
		})
	})

	Context("util.GetCopys", func() {
		It("Should work correctly", func() {
			p := copy.New(db)
			p.MustCreate()

			m := p.Fork()
			m.MustCreate()

			m2 := p.Fork()
			m2.MustCreate()

			ms, err := util.GetCopies(db, p)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(ms)).To(Equal(2))
		})

		It("Should not error if no results", func() {
			m := copy.New(db)

			ms, err := util.GetCopies(db, m)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(ms)).To(Equal(0))
		})
	})
})
