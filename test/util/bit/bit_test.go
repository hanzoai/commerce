package test

import (
	"testing"

	"github.com/hanzoai/commerce/util/bit"

	. "github.com/hanzoai/commerce/util/test/ginkgo"
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

	Measure("it should perform operations efficiently", func(b Benchmarker) {
		runtime := b.Time("runtime", func() {
			field := new(bit.Field)

			n := 0

			for {
				field.Set(A)
				field.Set(B)
				field.Set(C)

				n += 1

				if n == 100000 {
					break
				}
			}
		})

		Expect(runtime.Seconds()).Should(BeNumerically("<", 0.1))
	}, 10)
})
