package test

import (
	"hanzo.io/test/fixtures/user"

	. "hanzo.io/util/test/ginkgo"
)

var _ = Describe("models/mixin utils", func() {
	Context("Model.Zero", func() {
		It("should create a zero copy of an entity", func() {
			usr := user.New(db)
			usr.Name = "John"
			zero := usr.Zero().(*user.User)
			Expect(zero.Name).To(BeZero())
		})
	})

	Context("Model.Clone", func() {
		It("should create a clone of an entity", func() {
			usr := user.New(db)
			usr.Name = "John"
			clone := usr.Clone().(*user.User)
			Expect(clone.Name).To(Equal(usr.Name))
		})
	})

	Context("Model.CloneFromJSON", func() {
		It("should create clone using only JSON-exported fields", func() {
			usr := user.New(db)
			usr.Name = "John"
			usr.Hidden = "invisible"
			clone := usr.CloneFromJSON().(*user.User)
			Expect(clone.Name).To(Equal(usr.Name))
			Expect(clone.Hidden).To(BeZero())
		})
	})

	Context("Model.Slice", func() {
		It("should create a slice for entity", func() {
			usr := user.New(db)
			slice := usr.Slice()
			_, ok := slice.(*[]*user.User)
			Expect(ok).To(BeTrue())
		})
	})

	Context("Model.JSON", func() {
		It("should encode entity as JSON", func() {
			usr := user.New(db)
			bytes := usr.JSON()
			Expect(string(bytes)).To(Equal(`{
  "id": "",
  "createdAt": "0001-01-01T00:00:00Z",
  "updatedAt": "0001-01-01T00:00:00Z",
  "Name": "Nobody"
}`))
		})
	})
})
