package test

import (
	"errors"
	"testing"

	"appengine/aetest"
	gaed "appengine/datastore"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"crowdstart.io/datastore"
	"crowdstart.io/util/log"
)

var (
	ctx aetest.Context
	db  *datastore.Datastore
)

type TestStruct struct {
	Field string
}

func TestDatastore(t *testing.T) {
	RegisterFailHandler(Fail)
	log.Debug("1")
	RunSpecs(t, "Datastore test suite")
}

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	var err error
	ctx, err = aetest.NewContext(&aetest.Options{StronglyConsistentDatastore: true})
	Expect(err).NotTo(HaveOccurred())
	db = datastore.New(ctx)
	log.Debug("Hi")
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	err := ctx.Close()
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("EncodeId", func() {
	Context("Allocate id", func() {
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
	entity := TestStruct{"test-get-field"}
	kind := "test-get"
	var key string

	Context("With the wrapper's put", func() {
		BeforeEach(func() {
			var err error
			key, err = db.Put(kind, &entity)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not be empty", func() {
			var retrievedEntity TestStruct
			err := db.Get(key, &retrievedEntity)
			Expect(err).ToNot(HaveOccurred())
			Expect(retrievedEntity).ToNot(BeZero())
		})

		It("should equal what was inserted", func() {
			var retrievedEntity TestStruct
			err := db.Get(key, &retrievedEntity)
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

var _ = Describe("Put", func() {
	entity := TestStruct{"test-put-field"}
	kind := "test-put"

	var key string
	BeforeEach(func() {
		var err error
		key, err = db.Put(kind, &entity)
		Expect(err).NotTo(HaveOccurred())
		Expect(key).NotTo(BeZero())
	})

	Context("With the wrapper's get", func() {
		It("should be the same", func() {
			var retrievedEntity TestStruct
			err := db.Get(key, &retrievedEntity)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedEntity).To(Equal(entity))
		})
	})

	Context("With appengine's datastore.Get", func() {
		It("should be the same", func() {
			_key, err := db.DecodeKey(key)
			Expect(err).NotTo(HaveOccurred())

			var retrievedEntity TestStruct
			err = gaed.Get(ctx, _key, &retrievedEntity)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedEntity).To(Equal(entity))
		})
	})

	Context("With the query api", func() {
		It("should be the same", func() {
			var retrievedEntity TestStruct
			_, err := db.Query(kind).
				Filter("Field =", entity.Field).
				Run(ctx).
				Next(&retrievedEntity)

			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedEntity).To(Equal(entity))
		})
	})
})
