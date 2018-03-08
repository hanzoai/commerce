package test

import (
	"hanzo.io/test/fixtures/user"

	. "hanzo.io/util/test/ginkgo"
)

var _ = Describe("models/mixin Model", func() {
	Context("Model.Create", func() {
		It("should create entity in datastore", func() {
			// Create a new model and store using Model mixin
			usr := user.New(db)
			usr.Name = "Justin"
			Expect(usr.C).To(Equal(""))
			err := usr.Create()
			Expect(err).To(BeNil())
			Expect(usr.C).To(Equal("after"))

			// Should not allow multiple creates
			err = usr.Create()
			Expect(err).To(HaveOccurred())

			// Manually retrieve to ensure it was created properly
			usr2 := new(user.User)
			db.Get(usr.Key(), usr2)
			Expect(usr2.Name).To(Equal(usr.Name))
			Expect(usr2.C).To(Equal("before"))
		})
	})

	Context("Model.Update", func() {
		It("should update entity in datastore", func() {
			// Create a new model
			usr := user.New(db)
			usr.Name = "Justin"

			// Should not allow update w/o create
			err := usr.Update()
			Expect(err).To(HaveOccurred())

			// Create model
			err = usr.Create()
			Expect(err).To(BeNil())

			// Update it
			Expect(usr.U).To(Equal(""))
			usr.Name = "Justin2"
			err = usr.Update()
			Expect(err).To(BeNil())
			Expect(usr.U).To(Equal("after"))

			// Manually retrieve to ensure it was updated properly
			usr2 := user.New(db)
			err = usr2.SetKey(usr.Key())
			Expect(err).To(BeNil())
			err = usr2.Get()
			Expect(err).To(BeNil())
			Expect(usr2.Name).To(Equal(usr.Name))
			Expect(usr2.U).To(Equal("before"))

			// Second update should set U differently using previous model
			err = usr2.Update()
			Expect(err).To(BeNil())
			Expect(usr2.U).To(Equal("after2"))

			// Manually retrieve to ensure it was updated properly
			usr3 := new(user.User)
			db.Get(usr.Key(), usr3)
			Expect(usr3.Name).To(Equal(usr.Name))
			Expect(usr3.U).To(Equal("before2"))
		})
	})

	Context("Model.Delete", func() {
		It("should delete entity to datastore", func() {
			// Create a new model and store using Model mixin
			usr := user.New(db)
			usr.Name = "Justin"
			err := usr.Create()
			Expect(err).To(BeNil())

			Expect(usr.D).To(Equal(""))
			err = usr.Delete()
			Expect(err).To(BeNil())
			Expect(usr.D).To(Equal("after"))

			// Manually retrieve to ensure it was deleted properly
			usr2 := new(user.User)
			err = db.Get(usr.Key(), usr2)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Model.Put", func() {
		It("should save entity to datastore", func() {
			// Create a new model and store using Model mixin
			usr := user.New(db)
			usr.Name = "Justin"
			err := usr.Put()
			Expect(err).To(BeNil())

			// Manually retrieve to ensure it was saved properly
			usr2 := new(user.User)
			err = db.Get(usr.Key(), usr2)
			Expect(err).To(BeNil())
			Expect(usr2.Name).To(Equal(usr.Name))
		})
	})

	Context("Model.Get", func() {
		It("should retrieve entity from datastore", func() {
			// Manually create a new model and store in datastore
			usr := new(user.User)
			usr.Name = "Dustin"
			key, err := db.Put("user", usr)
			Expect(err).To(BeNil())

			// Retrieve model from datastore using Model mixin
			usr2 := user.New(db)
			err = usr2.Get(key)
			Expect(err).To(BeNil())
			Expect(usr2.Name).To(Equal(usr.Name))
		})
	})
})
