package test

import (
	"testing"

	"appengine/aetest"
	gaed "appengine/datastore"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"crowdstart.io/datastore"
	"crowdstart.io/util/log"
	"errors"

	"crowdstart.io/datastore"
)

type TestStruct struct {
	Field string
}

func TestDatastore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SuiteSuiteSuite")
}

func TestId(t *testing.T) {
	t.Skip()
	ctx, _ := aetest.NewContext(nil)
	defer ctx.Close()
	db := datastore.New(ctx)

	id := db.AllocateId("test")

	if id == 0 {
		t.Logf("Id is not valid, Expected ID to be non-0")
		t.Fail()
	}

	id1 := db.EncodeId("test", int64(12345))
	if id1 == "" {
		t.Logf("Encoding did not work")
		t.Fail()
	}

	id2 := db.EncodeId("test", int(12345))
	if id2 == "" {
		t.Logf("Encoding did not work")
		t.Fail()
	}

	id3 := db.EncodeId("test", "12345")
	if id3 == "" {
		t.Logf("Encoding did not work")
		t.Fail()
	}

	if id1 != id2 {
		t.Logf("Ids 1 & 2 should be equal. \n\t Expected: %#v \n\t Actual: %#v", id1, id2)
		t.Fail()
	}

	if id2 != id3 {
		t.Logf("Ids 2 & 3 should be equal. \n\t Expected: %#v \n\t Actual: %#v", id2, id3)
		t.Fail()
	}

	err := db.EncodeId("test", errors.New(""))
	if err != "" {
		t.Logf("EncodeId accepted invalid type")
		t.Fail()
	}
}

var _ = Describe("Get", func() {
	log.Info("Describe")

	var (
		ctx aetest.Context
		db  *datastore.Datastore
	)
	entity := TestStruct{"test-get-field"}
	kind := "test-get"

	BeforeEach(func() {
		log.Info("BeforeEach")

		var err error
		ctx, err = aetest.NewContext(nil)
		Expect(err).NotTo(HaveOccurred())
		db = datastore.New(ctx)
	})

	AfterEach(func() {
		log.Info("AfterEach")

		err := ctx.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("With the wrapper's put", func() {
		log.Info("Context")

		It("should not be empty", func() {
			log.Info("It")

			key, err := db.Put(kind, &entity)
			Expect(err).NotTo(HaveOccurred())

			va{ retrievedEntity TestStruct
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
		log.Info("Context 2")

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
