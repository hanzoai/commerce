package test

import (
	"errors"
	"testing"

	gaed "appengine/datastore"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"crowdstart.io/datastore"
	"github.com/zeekay/aetest"
)

var (
	ctx aetest.Context
	db  *datastore.Datastore
)

type Entity struct {
	Field string
}

func TestDatastore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Datastore test suite")
}

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	var err error
	ctx, err = aetest.NewContext(&aetest.Options{StronglyConsistentDatastore: true})
	Expect(err).NotTo(HaveOccurred())
	db = datastore.New(ctx)
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
		// Since this is testing a negative case, we disable warning
		// temporarily.
		BeforeEach(func() {
			db.Warn = false
		})
		AfterEach(func() {
			db.Warn = true
		})

		It("should error", func() {
			err := db.EncodeId("test", errors.New(""))
			Expect(err).To(Equal(""))
			Expect(err).NotTo(Equal(0))
		})
	})
})

var _ = Describe("Datastore.Get", func() {
	entity := &Entity{"test-get-field"}
	kind := "test-get"
	var key string

	Context("When storing entity with Datastore.Put", func() {
		BeforeEach(func() {
			var err error
			key, err = db.Put(kind, entity)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Retrieved entity should not be empty", func() {
			retrievedEntity := &Entity{}
			err := db.Get(key, retrievedEntity)
			Expect(err).ToNot(HaveOccurred())
			Expect(retrievedEntity).ToNot(BeZero())
		})

		It("Retrieved entity should equal what was inserted", func() {
			retrievedEntity := &Entity{}
			err := db.Get(key, retrievedEntity)
			Expect(err).ToNot(HaveOccurred())
			Expect(retrievedEntity).To(Equal(entity))
		})
	})

	Context("When storing entity with appengine/datastore", func() {
		retrievedEntity := &Entity{}
		BeforeEach(func() {
			key := gaed.NewKey(ctx, kind, "key", 0, nil)
			_, err := gaed.Put(ctx, key, entity)
			Expect(err).ToNot(HaveOccurred())

			err = db.Get(key.Encode(), retrievedEntity)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Retrieved entity should not be empty", func() {
			Expect(retrievedEntity).ToNot(BeZero())
		})

		It("Retrieved entity should equal what was inserted", func() {
			Expect(retrievedEntity).To(Equal(entity))
		})
	})
})

var _ = Describe("Put", func() {
	kind := "test-put"

	Context("With the wrapper's get", func() {
		It("should store entity successfully", func() {
			a := &Entity{"test-wrapper-put"}
			b := &Entity{}

			// Store entity
			key, err := db.Put(kind, a)
			Expect(err).NotTo(HaveOccurred())

			// Try to retrieve entity
			err = db.Get(key, b)
			Expect(err).NotTo(HaveOccurred())

			Expect(a).To(Equal(b))
		})
	})

	Context("With appengine's datastore.Get", func() {
		It("should be the same", func() {
			a := &Entity{"test-appengine-put"}
			b := &Entity{}

			// Store entity
			key, err := db.Put(kind, a)
			Expect(err).NotTo(HaveOccurred())

			// Try to retrieve entity
			_key, err := db.DecodeKey(key)
			Expect(err).NotTo(HaveOccurred())

			err = gaed.Get(ctx, _key, b)
			Expect(err).NotTo(HaveOccurred())
			Expect(a).To(Equal(b))
		})
	})

	Context("With the query api", func() {
		It("should be the same", func() {
			a := &Entity{"test-query-put"}
			b := &Entity{}
			// Store entity
			_, err := db.Put(kind, a)
			Expect(err).NotTo(HaveOccurred())
			_, err = db.Query(kind).
				Filter("Field =", a.Field).
				Run(ctx).
				Next(b)

			Expect(err).NotTo(HaveOccurred())
			Expect(a).To(Equal(b))
		})
	})
})
