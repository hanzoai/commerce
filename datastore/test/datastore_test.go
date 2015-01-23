package test

import (
	"testing"
	"errors"

	"appengine/aetest"
	gaed "appengine/datastore"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"crowdstart.io/datastore"
)

type TestStruct struct {
	Field string
}

func TestDatastore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SuiteSuiteSuite")
}

var _ = Describe("EncodeId", func() {
	var (
		ctx aetest.Context
		db *datastore.Datastore
	)
	BeforeEach(func() {
		var err error
		ctx, err = aetest.NewContext(nil)
		Expect(err).NotTo(HaveOccurred())
		db = datastore.New(ctx)
	})
	AfterEach(func() {
		ctx.Close()
	})
	
	Context("Allocated id", func() {
		It("should be non-zero", func() {
			id := db.AllocateId("test")
			Expect(id).NotTo(Equal(0))
		})
	})

	Context("Encoding int64", func() {
		It("should not be an empty string", func() {
			id := db.EncodeId("test", int64(12345))
			Expect(id).NotTo(Equal(""))
		})
	})

	Context("Encoding int", func() {
		It("should not be an empty string", func() {
			id := db.EncodeId("test", int(12345))
			Expect(id).NotTo(Equal(""))
		})
	})
	
	Context("Encoding string", func() {
		It("should not be an empty string", func() {
			id := db.EncodeId("test", "12345")
			Expect(id).NotTo(Equal(""))
		})
	})
	
	Context("Encoded int64 and int", func() {
		It("should be the same", func() {
			id1 := db.EncodeId("test", int64(12345))
			id2 := db.EncodeId("test", int(12345))
			Expect(id1).To(Equal(id2))
		})
	})
	
	Context("Encoded int and string", func() {
		It("should be the same", func() {
			id1 := db.EncodeId("test", int(12345))
			id2 := db.EncodeId("test", "12345")
			Expect(id1).To(Equal(id2))
		})
	})
	
	Context("Encoding bad types", func() {
		It("should error", func() {
			err := db.EncodeId("test", errors.New(""))
			Expect(err).To(Equal(""))
		})
	})
})

var _ = Describe("Get", func() {
	var (
		ctx aetest.Context
		db  *datastore.Datastore
	)
	entity := TestStruct{"test-get-field"}
	kind := "test-get"

	BeforeEach(func() {
		var err error
		ctx, err = aetest.NewContext(nil)
		Expect(err).NotTo(HaveOccurred())
		db = datastore.New(ctx)
	})

	AfterEach(func() {
		err := ctx.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("With the wrapper's put", func() {
		It("should not be empty", func() {
			key, err := db.Put(kind, &entity)
			Expect(err).NotTo(HaveOccurred())

			var retrievedEntity TestStruct
			err = db.Get(key, &retrievedEntity)
			Expect(err).ToNot(HaveOccurred())
			Expect(retrievedEntity).ToNot(BeZero())
		})

		It("should equal what was inserted", func() {
			key, err := db.Put(kind, &entity)
			Expect(err).NotTo(HaveOccurred())

			var retrievedEntity TestStruct
			err = db.Get(key, &retrievedEntity)
			Expect(err).ToNot(HaveOccurred())
			Expect(retrievedEntity).To(Equal(entity))
		})
	})

	Context("With appengine's datastore.put", func() {
		var retrievedEntity TestStruct
		BeforeEach(func() {
			key := gaed.NewKey(ctx, kind, "key", 0, nil)
			_, err := gaed.Put(ctx, key, &entity)
			Expect(err).ToNot(HaveOccurred())

			err = db.Get(key.Encode(), &retrievedEntity)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should not be empty", func() {
			Expect(retrievedEntity).ToNot(BeZero())
		})
		It("should equal what was inserted", func() {
			Expect(retrievedEntity).To(Equal(entity))
		})
	})
})
