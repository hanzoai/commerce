package test

import (
	"hanzo.io/test/fixtures/user"

	. "hanzo.io/util/test/ginkgo"
)

var _ = Describe("models/mixin Query", func() {
	Context("Query.Ancestor", func() {
		It("should set ancestor in query", func() {
			usr := user.New(db)
			usr.Name = "Ancestor"
			usr.MustPut()

			usr2 := user.New(db)
			usr2.Name = "Child"
			usr2.Parent = usr.Key()
			usr2.MustPut()

			usr3 := user.New(db)
			usr3.Name = "Not Child"
			usr3.MustPut()

			entities, err := user.Query(db).Ancestor(usr.Key()).GetEntities()
			Expect(err).To(BeNil())
			Expect(len(entities)).To(Equal(2)) // Why is this not 1?
		})
	})

	Context("Query.Limit", func() {
		It("should set limit on query", func() {
			usr := user.New(db)
			usr.Name = "Foo"
			usr.MustPut()

			usr2 := user.New(db)
			usr2.Name = "Foo"
			usr2.Parent = usr.Key()
			usr2.MustPut()

			entities, err := user.Query(db).Limit(1).GetEntities()
			Expect(err).To(BeNil())
			Expect(len(entities)).To(Equal(1))
		})
	})

	Context("Query.Offset", func() {
		It("should set offset query", func() {
			usr := user.New(db)
			usr.Name = "Bar"
			usr.MustPut()

			usr2 := user.New(db)
			usr2.Name = "Bar"
			usr2.Parent = usr.Key()
			usr2.MustPut()

			entities, err := user.Query(db).Filter("Name=", "Bar").Offset(1).GetEntities()
			Expect(err).To(BeNil())
			Expect(len(entities)).To(Equal(1))
		})
	})
})
