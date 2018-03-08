package test

import (
	"strconv"
	"testing"

	aeds "google.golang.org/appengine/datastore"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"hanzo.io/datastore"
	"hanzo.io/log"
	"hanzo.io/util/test/ae"
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

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	var err error
	ctx = ae.NewContext()
	Expect(err).NotTo(HaveOccurred())
	db = datastore.New(ctx)
})

// Tear-down appengine context
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
	Context("Key encoded with appengine", func() {
		It("should be the same", func() {
			key := aeds.NewKey(ctx, kind, "decodekey-testkey", 0, nil)
			decodedKey, err := db.DecodeKey(key.Encode())
			Expect(err).ToNot(HaveOccurred())
			Expect(decodedKey).To(Equal(key))
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
	var key *aeds.Key

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
			key := aeds.NewKey(ctx, kind, "key", 0, nil)
			_, err := aeds.Put(ctx, key, entity)
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

	Context("With appengine's datastore.Get", func() {
		It("should be the same", func() {
			a := &Entity{"test-appengine-put"}
			b := &Entity{}

			// Store entity
			key, err := db.Put(kind, a)
			Expect(err).NotTo(HaveOccurred())

			err = aeds.Get(ctx, key, b)
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
	// Context("With Datastore.PutMulti", func() {
	// 	It("should be the same", func() {
	// 		a := make([]Entity, 10)
	// 		b := make([]interface{}, len(a))
	// 		for i, _ := range a {
	// 			entity := Entity{str(i)}
	// 			a[i] = entity
	// 			b[i] = &entity
	// 		}
	// 		keys, err := db.PutMulti(kind, b)
	// 		Expect(err).ToNot(HaveOccurred())

	// 		c := make([]Entity, len(a))
	// 		err = db.GetMulti(keys, c)
	// 		Expect(err).ToNot(HaveOccurred())
	// 		Expect(c).To(Equal(a))
	// 		Expect(c).To(HaveLen(len(a)))
	// 	})
	// })

	Context("With appengine's datastore.PutMulti", func() {
		It("should be the same", func() {
			a := make([]Entity, 10)
			keys := func() []*aeds.Key {
				keys := make([]*aeds.Key, len(a))
				for i, _ := range keys {
					a[i].Field = str(i)
					aKey := aeds.NewKey(ctx, kind, str(i), 0, nil)
					keys[i] = aKey
				}
				keys, err := aeds.PutMulti(ctx, keys, a)
				Expect(err).ToNot(HaveOccurred())
				return keys
			}()

			b := make([]Entity, 10)
			err := db.GetMulti(keys, b)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(Equal(a))
			Expect(b).To(HaveLen(len(a)))
		})
	})
})

// var _ = Describe("Datastore.PutMulti", func() {
// 	kind := "datastore-putmulti-test"

// 	Context("With Datastore.Get", func() {
// 		It("should be the same", func() {
// 			a := make([]Entity, 10)
// 			b := make([]interface{}, len(a))
// 			for i, _ := range a {
// 				entity := Entity{str(i)}
// 				a[i] = entity
// 				b[i] = &entity
// 			}
// 			keys, err := db.PutMulti(kind, b)
// 			Expect(err).ToNot(HaveOccurred())

// 			c := new(Entity)
// 			err = db.Get(keys[len(a)-1], c)
// 			Expect(err).ToNot(HaveOccurred())
// 			Expect(*c).To(Equal(a[len(a)-1]))
// 		})
// 	})

// 	Context("With Datastore.GetMulti", func() {
// 		It("should be the same", func() {
// 			a := make([]Entity, 10)
// 			b := make([]interface{}, len(a))
// 			for i, _ := range a {
// 				entity := Entity{str(i)}
// 				a[i] = entity
// 				b[i] = &entity
// 			}
// 			keys, err := db.PutMulti(kind, b)
// 			Expect(err).ToNot(HaveOccurred())

// 			c := make([]Entity, len(a))
// 			err = db.GetMulti(keys, c)
// 			Expect(err).ToNot(HaveOccurred())
// 			Expect(c).To(Equal(a))
// 			Expect(c).To(HaveLen(len(a)))
// 		})
// 	})

// 	Context("With appengine's datastore.GetMulti", func() {
// 		It("should be the same", func() {
// 			a := make([]Entity, 10)
// 			b := make([]interface{}, len(a))
// 			for i, _ := range a {
// 				entity := Entity{str(i)}
// 				a[i] = entity
// 				b[i] = &entity
// 			}
// 			keys, err := db.PutMulti(kind, b)
// 			Expect(err).ToNot(HaveOccurred())

// 			c := make([]Entity, len(a))
// 			err = aeds.GetMulti(ctx, keys, c)
// 			Expect(err).ToNot(HaveOccurred())
// 			Expect(c).To(Equal(a))
// 			Expect(c).To(HaveLen(len(a)))
// 		})
// 	})
// })
