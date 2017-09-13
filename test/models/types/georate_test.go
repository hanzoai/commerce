package test

import (
	"hanzo.io/models/types/currency"
	"hanzo.io/models/types/georate"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("models/types/georate", func() {
	Context("New", func() {
		It("Should Clean Input (City)", func() {
			gr := georate.New("us", "ks", "city", "postal codes", 0.1, 1)
			Expect(gr.Country).To(Equal("US"))
			Expect(gr.State).To(Equal("KS"))
			Expect(gr.City).To(Equal("CITY"))
			Expect(gr.PostalCodes).To(Equal(""))
			Expect(gr.Percent).To(Equal(0.1))
			Expect(gr.Cost).To(Equal(currency.Cents(1)))
		})

		It("Should Clean Input (Postal Code)", func() {
			gr := georate.New("us", "ks", "", "postal codes", 0.1, 1)
			Expect(gr.Country).To(Equal("US"))
			Expect(gr.State).To(Equal("KS"))
			Expect(gr.City).To(Equal(""))
			Expect(gr.PostalCodes).To(Equal("POSTALCODES"))
			Expect(gr.Percent).To(Equal(0.1))
			Expect(gr.Cost).To(Equal(currency.Cents(1)))
		})

		It("Should Only Accept Real Countries", func() {
			gr := georate.New("zzz", "asv", "city", "postal codes", 0.1, 1)
			Expect(gr.Country).To(Equal(""))
			Expect(gr.State).To(Equal(""))
			Expect(gr.City).To(Equal(""))
			Expect(gr.PostalCodes).To(Equal(""))
			Expect(gr.Percent).To(Equal(0.1))
			Expect(gr.Cost).To(Equal(currency.Cents(1)))
		})

		It("Should Only Accept Real Country/State Combos", func() {
			gr := georate.New("us", "asv", "city", "postal codes", 0.1, 1)
			Expect(gr.Country).To(Equal("US"))
			Expect(gr.State).To(Equal(""))
			Expect(gr.City).To(Equal(""))
			Expect(gr.PostalCodes).To(Equal(""))
			Expect(gr.Percent).To(Equal(0.1))
			Expect(gr.Cost).To(Equal(currency.Cents(1)))
		})
	})

	Context("Match", func() {
		It("L0 (Wild Card) Match", func() {
			gr := georate.New("", "", "", "", 0.1, 1)
			isMatch, level := gr.Match("us", "ks", "", "66212")
			Expect(isMatch).To(BeTrue())
			Expect(level).To(Equal(0))
		})

		It("L0 No Match", func() {
			gr := georate.New("us", "ks", "", "66213", 0.1, 1)
			isMatch, level := gr.Match("gb", "bkm", "", "sl8")
			Expect(isMatch).To(BeFalse())
			Expect(level).To(Equal(0))
		})

		It("L1 (Country) Match", func() {
			gr := georate.New("us", "", "", "", 0.1, 1)
			isMatch, level := gr.Match("us", "ks", "", "66212")
			Expect(isMatch).To(BeTrue())
			Expect(level).To(Equal(1))
		})

		It("L1 (Country) Partial Match", func() {
			gr := georate.New("us", "mo", "", "", 0.1, 1)
			isMatch, level := gr.Match("us", "ks", "", "66212")
			Expect(isMatch).To(BeTrue())
			Expect(level).To(Equal(1))
		})

		It("L2 (Country + State) Match", func() {
			gr := georate.New("us", "ks", "", "", 0.1, 1)
			isMatch, level := gr.Match("us", "ks", "", "66212")
			Expect(isMatch).To(BeTrue())
			Expect(level).To(Equal(2))
		})

		It("L2 (Country + State) Partial Match", func() {
			gr := georate.New("us", "ks", "", "66213", 0.1, 1)
			isMatch, level := gr.Match("us", "ks", "", "66212")
			Expect(isMatch).To(BeTrue())
			Expect(level).To(Equal(2))
		})

		It("L3 (Country + State + City) Match", func() {
			gr := georate.New("us", "ks", "Overland Park", "66212", 0.1, 1)
			isMatch, level := gr.Match("us", "ks", "overland park", "66212")
			Expect(isMatch).To(BeTrue())
			Expect(level).To(Equal(3))
		})

		It("L3 (Country + State + Postal Code) Match", func() {
			gr := georate.New("us", "ks", "", "66212", 0.1, 1)
			isMatch, level := gr.Match("us", "ks", "", "66212")
			Expect(isMatch).To(BeTrue())
			Expect(level).To(Equal(3))
		})
	})
})
