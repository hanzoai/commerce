package test

import (
	"errors"
	"strconv"
	"testing"

	aeds "appengine/datastore"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/zeekay/aetest"

	"crowdstart.io/datastore"
	"crowdstart.io/util/log"
)

func TestDatastore(t *testing.T) {
	log.SetVerbose(testing.Verbose())
	RegisterFailHandler(Fail)
	RunSpecs(t, "datastore")
}

var (
	ctx aetest.Context
	db  *datastore.Datastore
)

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
			key := db.EncodeId("test", errors.New(""))
			Expect(key).To(Equal(""))
			Expect(key).NotTo(Equal(0))
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
				Run(ctx).
				Next(b)

			Expect(err).NotTo(HaveOccurred())
			Expect(a).To(Equal(b))
		})
	})
})

var _ = Describe("Datastore.GetKind", func() {
	kind := "datastore-getkey-test"
	Context("With Datastore.PutKind", func() {
		It("should be the same", func() {
			key := "test-datastore-getkey_"
			a := &Entity{"test-datastore-getkey"}
			_, err := db.PutKind(kind, key, a)
			Expect(err).ToNot(HaveOccurred())

			b := &Entity{}
			err = db.GetKind(kind, key, b)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(Equal(a))
		})
	})

	Context("With appengine's datastore.Put", func() {
		It("should be the same", func() {
			a := &Entity{"test-appengine-put"}
			key := aeds.NewKey(ctx, kind, "test-key", 0, nil)
			_, err := aeds.Put(ctx, key, a)
			Expect(err).ToNot(HaveOccurred())

			b := &Entity{}
			err = db.GetKind(kind, "test-key", b)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(Equal(a))
		})
	})
})

var _ = Describe("Datastore.PutKind", func() {
	kind := "datastore-putkey-test"
	Context("With Datastore.GetKind", func() {
		It("should be the same", func() {
			a := &Entity{"test-datastore"}
			key := "test-datastore-putkey-getkey"
			_, err := db.PutKind(kind, key, a)
			Expect(err).ToNot(HaveOccurred())

			b := &Entity{}
			err = db.GetKind(kind, key, b)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(Equal(a))
		})
	})

	Context("With appengine's datastore.Get", func() {
		It("should be the same", func() {
			a := &Entity{"test-datastore"}
			key := "test-datastore-putkey-getkey"
			_, err := db.PutKind(kind, key, a)
			Expect(err).ToNot(HaveOccurred())

			b := &Entity{}
			aKey := aeds.NewKey(ctx, kind, key, 0, nil)
			err = aeds.Get(ctx, aKey, b)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(Equal(a))
		})
	})
})

var _ = Describe("Datastore.GetMulti", func() {
	kind := "datastore-getmulti-test"
	Context("With Datastore.PutMulti", func() {
		It("should be the same", func() {
			a := make([]Entity, 10)
			b := make([]interface{}, len(a))
			for i, _ := range a {
				entity := Entity{str(i)}
				a[i] = entity
				b[i] = &entity
			}
			keys, err := db.PutMulti(kind, b)
			Expect(err).ToNot(HaveOccurred())

			c := make([]Entity, len(a))
			err = db.GetMulti(keys, c)
			Expect(err).ToNot(HaveOccurred())
			Expect(c).To(Equal(a))
			Expect(c).To(HaveLen(len(a)))
		})
	})

	Context("With appengine's datastore.PutMulti", func() {
		It("should be the same", func() {
			a := make([]Entity, 10)
			keys := func() []string {
				keys := make([]*aeds.Key, len(a))
				for i, _ := range keys {
					a[i].Field = str(i)
					aKey := aeds.NewKey(ctx, kind, str(i), 0, nil)
					keys[i] = aKey
				}
				_, err := aeds.PutMulti(ctx, keys, a)
				Expect(err).ToNot(HaveOccurred())

				strKeys := make([]string, len(keys))
				for i, key := range keys {
					strKeys[i] = key.Encode()
				}
				return strKeys
			}()

			b := make([]Entity, 10)
			err := db.GetMulti(keys, b)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(Equal(a))
			Expect(b).To(HaveLen(len(a)))
		})
	})
})

var _ = Describe("Datastore.PutMulti", func() {
	kind := "datastore-putmulti-test"

	Context("With Datastore.Get", func() {
		It("should be the same", func() {
			a := make([]Entity, 10)
			b := make([]interface{}, len(a))
			for i, _ := range a {
				entity := Entity{str(i)}
				a[i] = entity
				b[i] = &entity
			}
			keys, err := db.PutMulti(kind, b)
			Expect(err).ToNot(HaveOccurred())

			c := new(Entity)
			err = db.Get(keys[len(a)-1], c)
			Expect(err).ToNot(HaveOccurred())
			Expect(*c).To(Equal(a[len(a)-1]))
		})
	})

	Context("With Datastore.GetMulti", func() {
		It("should be the same", func() {
			a := make([]Entity, 10)
			b := make([]interface{}, len(a))
			for i, _ := range a {
				entity := Entity{str(i)}
				a[i] = entity
				b[i] = &entity
			}
			keys, err := db.PutMulti(kind, b)
			Expect(err).ToNot(HaveOccurred())

			c := make([]Entity, len(a))
			err = db.GetMulti(keys, c)
			Expect(err).ToNot(HaveOccurred())
			Expect(c).To(Equal(a))
			Expect(c).To(HaveLen(len(a)))
		})
	})

	Context("With appengine's datastore.GetMulti", func() {
		It("should be the same", func() {
			a := make([]Entity, 10)
			b := make([]interface{}, len(a))
			for i, _ := range a {
				entity := Entity{str(i)}
				a[i] = entity
				b[i] = &entity
			}
			keys, err := db.PutMulti(kind, b)
			Expect(err).ToNot(HaveOccurred())

			c := make([]Entity, len(a))
			err = aeds.GetMulti(ctx, keys, c)
			Expect(err).ToNot(HaveOccurred())
			Expect(c).To(Equal(a))
			Expect(c).To(HaveLen(len(a)))
		})
	})
})

var _ = Describe("Datastore.GetKindMulti", func() {
	kind := "datastore-GetMultiKey-test"

	Context("With Datastore.PutKind", func() {
		It("should be the same", func() {
			a := make([]Entity, 10)
			keys := make([]string, len(a))
			for i, _ := range a {
				a[i].Field = str(i)
				keys[i] = a[i].Field
				_, err := db.PutKind(kind, a[i].Field, &a[i])
				Expect(err).ToNot(HaveOccurred())
			}

			b := make([]Entity, len(a))
			err := db.GetKindMulti(kind, keys, b)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(Equal(a))
		})
	})

	Context("With appengine's datastore.PutMulti", func() {
		It("should be the same", func() {
			a := make([]Entity, 10)
			keys := func() []string {
				for i, _ := range a {
					entity := Entity{str(i)}
					a[i] = entity
				}
				_keys := make([]string, len(a))
				keys := make([]*aeds.Key, len(a))
				for i := 30; i < 30+len(a); i++ {
					keys[i-30] = aeds.NewKey(ctx, kind, str(i), 0, nil)
					_keys[i-30] = str(i)
				}
				_, err := aeds.PutMulti(ctx, keys, a)
				Expect(err).ToNot(HaveOccurred())
				return _keys
			}()

			c := make([]Entity, len(a))
			err := db.GetKindMulti(kind, keys, c)
			Expect(err).ToNot(HaveOccurred())
			Expect(c).To(Equal(a))
		})
	})
})

var _ = Describe("Datastore.PutKeyMulti", func() {
	kind := "datastore-PutKeyMulti-test"
	Context("With Datastore.GetKindMulti", func() {
		It("should be the same", func() {
			a := make([]Entity, 3)
			b := make([]interface{}, len(a))
			keys := make([]string, len(a))
			_keys := make([]interface{}, len(a))
			for i, _ := range a {
				a[i].Field = str(i)
				b[i] = &a[i]
				keys[i] = a[i].Field
				_keys[i] = keys[i]
			}
			_, err := db.PutKindMulti(kind, _keys, b)
			Expect(err).ToNot(HaveOccurred())

			c := make([]Entity, len(keys))
			err = db.GetKindMulti(kind, keys, c)
			Expect(err).ToNot(HaveOccurred())
			Expect(c).To(Equal(a))
		})
	})
})
