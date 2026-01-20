package test

import (
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/util/test/ae"
)

func Test(t *testing.T) {
	log.SetVerbose(testing.Verbose())
	RegisterFailHandler(Fail)
	RunSpecs(t, "datastore")
}

var (
	ctx ae.Context
	db  *datastore.Datastore
)

// Setup test context and datastore before tests
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	db = datastore.New(ctx)
})

// Tear-down test context
var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("Key", func() {
	Context("AllocateID", func() {
		It("should be non-zero", func() {
			id := db.AllocateID("test", nil)
			Expect(id).NotTo(Equal(0))
		})
	})

	Context("Integer key", func() {
		It("should create key from int", func() {
			ickey := db.NewIncompleteKey("foo", nil)
			aekey, _ := db.NewKeyFromInt("foo", 10, nil)
			Expect(aekey).NotTo(Equal(ickey))
		})
	})

	Context("String key", func() {
		It("should create key from string", func() {
			ickey := db.NewIncompleteKey("foo", nil)
			aekey := db.NewKeyFromString("foo", "bar", nil)
			Expect(aekey).NotTo(Equal(ickey))
		})
	})
})

var _ = Describe("Datastore.DecodeKey", func() {
	kind := "decodekey-test"
	Context("Key encoded with datastore", func() {
		It("should be the same", func() {
			key := db.NewKeyFromString(kind, "decodekey-testkey", nil)
			decodedKey, err := db.DecodeKey(key.Encode())
			Expect(err).ToNot(HaveOccurred())
			Expect(decodedKey.Kind()).To(Equal(key.Kind()))
			Expect(decodedKey.StringID()).To(Equal(key.StringID()))
		})
	})
})

type Entity struct {
	Field string
}

func str(i int) string {
	return strconv.Itoa(i)
}

var _ = Describe("Datastore.Get", func() {
	entity := &Entity{"test-get-field"}
	kind := "test-get"
	var key datastore.Key

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

	Context("When storing entity with Put and GetById", func() {
		retrievedEntity := &Entity{}
		BeforeEach(func() {
			key := db.NewKeyFromString(kind, "key", nil)
			_, err := db.PutWithKey(key, entity)
			Expect(err).ToNot(HaveOccurred())

			err = db.GetById(key.Encode(), retrievedEntity)
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

	Context("With Datastore.Get", func() {
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

	Context("With Get after Put", func() {
		It("should be the same", func() {
			a := &Entity{"test-put-get"}
			b := &Entity{}

			// Store entity
			key, err := db.Put(kind, a)
			Expect(err).NotTo(HaveOccurred())

			err = db.Get(key, b)
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
				Run().
				Next(b)

			Expect(err).NotTo(HaveOccurred())
			Expect(a).To(Equal(b))
		})
	})
})

var _ = Describe("Datastore.GetMulti", func() {
	kind := "datastore-getmulti-test"

	Context("With Put and GetMulti", func() {
		It("should be the same", func() {
			a := make([]Entity, 10)
			keys := make([]datastore.Key, len(a))
			for i := range keys {
				a[i].Field = str(i)
				key := db.NewKeyFromString(kind, str(i), nil)
				keys[i] = key
				_, err := db.PutWithKey(key, &a[i])
				Expect(err).ToNot(HaveOccurred())
			}

			b := make([]Entity, 10)
			err := db.GetMulti(keys, b)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(Equal(a))
			Expect(b).To(HaveLen(len(a)))
		})
	})
})
