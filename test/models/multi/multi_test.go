package test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"hanzo.io/models/multi"
	"hanzo.io/test/fixtures/user"
)

var _ = Describe("models/multi", func() {
	Context("multi.Put", func() {
		It("should save entity to datastore", func() {
			// Create a new user and store using Model mixin
			usr := user.New(db)
			usr.Name = "Justin"
			usr2 := user.New(db)
			usr2.Name = "Todd"
			multi.MustPut([]interface{}{usr, usr2})

			// Manually retrieve to ensure it was saved properly
			usr3 := new(user.User)
			db.Get(usr2.Key(), usr3)
			Expect(usr2.Name).To(Equal(usr3.Name))
		})
	})

	Context("multi.Create", func() {
		It("should save entity to datastore and call create hooks", func() {
			// Create a new user and store using Model mixin
			usr := user.New(db)
			usr.Name = "Justin"
			usr2 := user.New(db)
			usr2.Name = "Todd"
			multi.MustCreate([]interface{}{usr, usr2})

			// Manually retrieve to ensure it was saved properly
			usr3 := new(user.User)
			db.Get(usr2.Key(), usr3)
			Expect(usr2.Name).To(Equal(usr3.Name))
		})
	})
})
