package test

import (
	"testing"

	"crowdstart.io/util/bit"

	. "crowdstart.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("util/bit", t)
}

const (
	A bit.Mask = 1 << iota
	B
	C
)

var _ = Describe("Field", func() {
	It("Should be able to set Mask", func() {
		field := new(bit.Field)

		field.Set(A)
		field.Set(B)

		Expect(field.Has(A)).To(Equal(true))
		Expect(field.Has(B)).To(Equal(true))
		Expect(field.Has(C)).To(Equal(false))
	})

	It("Should be able to remove Mask", func() {
		field := new(bit.Field)

		field.Set(A)
		field.Set(B)
		field.Set(C)

		field.Del(B)

		Expect(field.Has(A)).To(Equal(true))
		Expect(field.Has(B)).To(Equal(false))
		Expect(field.Has(C)).To(Equal(true))
	})
})
