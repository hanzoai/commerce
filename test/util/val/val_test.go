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

// These tests also check the reflect logic for supporting types derived from base types
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

var _ = Describe("IsEmail", func() {
	It("Should Fail for Non-Emails", func() {
		vs := ValStruct{StringField: "Not An Email"}
		v := val.New(&vs)

		errs := v.Check("StringField").IsEmail().Execute()
		Expect(len(errs)).To(Equal(1))
	})

	It("Should Work for Email String", func() {
		vs := ValStruct{StringField: "is@a.email"}
		v := val.New(&vs)

		errs := v.Check("StringField").IsEmail().Execute()
		Expect(len(errs)).To(Equal(0))
	})
})

var _ = Describe("IsPassword", func() {
	It("Should Fail for Short Passwords", func() {
		vs := ValStruct{StringField: "Short"}
		v := val.New(&vs)

		errs := v.Check("StringField").IsPassword().Execute()
		Expect(len(errs)).To(Equal(1))
	})

	It("Should Work for Long Passwords", func() {
		vs := ValStruct{StringField: "LessShort"}
		v := val.New(&vs)

		errs := v.Check("StringField").IsPassword().Execute()
		Expect(len(errs)).To(Equal(0))
	})
})

var _ = Describe("MinLength", func() {
	It("Should Fail for strings of lengths shorter than required", func() {
		vs := ValStruct{StringField: "Short"}
		v := val.New(&vs)

		errs := v.Check("StringField").MinLength(6).Execute()
		Expect(len(errs)).To(Equal(1))
	})

	It("Should Work for strings of lengths longer or equal to required", func() {
		vs := ValStruct{StringField: "Short"}
		v := val.New(&vs)

		errs := v.Check("StringField").MinLength(5).Execute()
		Expect(len(errs)).To(Equal(0))
	})
})

var _ = Describe("Matches", func() {
	It("Should Fail for strings that don't match required", func() {
		vs := ValStruct{StringField: "Short"}
		v := val.New(&vs)

		errs := v.Check("StringField").Matches("Long", "Longer").Execute()
		Expect(len(errs)).To(Equal(1))
	})

	It("Should Work for strings that match required", func() {
		vs := ValStruct{StringField: "Short"}
		v := val.New(&vs)

		errs := v.Check("StringField").Matches("Short", "Shorter").Execute()
		Expect(len(errs)).To(Equal(0))
	})
})
