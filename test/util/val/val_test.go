package test

import (
	"testing"

	"crowdstart.io/util/test/ae"
	"crowdstart.io/util/val"

	. "crowdstart.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("util/val", t)
}

var (
	ctx ae.Context
)

// Setup appengine context
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

type KindOfAString string

type ValStruct struct {
	StringField        string
	KindOfAStringField string
	IntField           int
	FloatField         float64
}

var _ = Describe("Exists", func() {
	It("Should Fail for Empty String", func() {
		vs := ValStruct{}
		v := val.New(&vs)

		errs := v.Check("StringField").Exists().Execute()
		Expect(len(errs)).To(Equal(1))
	})

	It("Should Fail for NonStrings", func() {
		vs := ValStruct{IntField: 123, FloatField: 123.0}
		v := val.New(&vs)

		errs := v.Check("IntField").Exists().
			Check("FloatField").Exists().Execute()
		Expect(len(errs)).To(Equal(2))
	})

	It("Should Work for NonEmpty String", func() {
		vs := ValStruct{StringField: "123"}
		v := val.New(&vs)

		errs := v.Check("StringField").Exists().Execute()
		Expect(len(errs)).To(Equal(0))
	})

	It("Should Work for NonEmpty StringDerived Type", func() {
		vs := ValStruct{KindOfAStringField: "123"}
		v := val.New(&vs)

		errs := v.Check("KindOfAStringField").Exists().Execute()
		Expect(len(errs)).To(Equal(0))
	})
})
