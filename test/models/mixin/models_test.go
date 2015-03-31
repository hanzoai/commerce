package test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("models/mixin Model", func() {
	Context("Model.Put", func() {
		It("should save entity to datastore", func() {
			// Create a new user and store using Model mixin
			user := newUser(db)
			user.Name = "Justin"
			user.Put()

			// Manually retrieve to ensure it was saved properly
			user2 := new(User)
			db.Get(user.Key(), user2)
			Expect(user2.Name).To(Equal(user.Name))
		})
	})

	Context("Model.Get", func() {
		It("should retrieve entity from datastore", func() {
			// Manually create a new user and store in datastore
			user := new(User)
			user.Name = "Dustin"
			key, err := db.Put("user", user)
			Expect(err).NotTo(HaveOccurred())

			// Retrieve user from datastore using Model mixin
			user2 := newUser(db)
			user2.Get(key)
			Expect(user2.Name).To(Equal(user.Name))
		})
	})
})
