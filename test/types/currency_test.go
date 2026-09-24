package test

import (
	"math/big"

	"github.com/hanzoai/money"

	. "github.com/hanzoai/commerce/models/types/currency"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("models/types/currency", func() {
	Context("Type.IsZero", func() {
		It("Should return true for certain currencies and false for others", func() {
			Expect(JPY.IsZeroDecimal()).To(BeTrue())
			Expect(USD.IsZeroDecimal()).To(BeFalse())
		})
	})

	Context("Type.ToString/Type.Symbol/Type.TopStringNoSymbol", func() {
		It("Should render positive currency cents correctly", func() {
			// Non Zero Decimal
			s1 := USD.ToString(Cents(10000))
			Expect(s1).To(Equal("$100.00"))
			sym1 := USD.Symbol()
			noSym1 := USD.ToStringNoSymbol(Cents(10000))
			Expect(s1).To(Equal(sym1 + noSym1))

			// Zero Decimal
			s2 := JPY.ToString(Cents(10000))
			Expect(s2).To(Equal("¥10000"))
			sym2 := JPY.Symbol()
			noSym2 := JPY.ToStringNoSymbol(Cents(10000))
			Expect(s2).To(Equal(sym2 + noSym2))
		})

		It("Should render negative currency cents correctly", func() {
			// Non Zero Decimal
			Expect(USD.ToString(Cents(-10000))).To(Equal("-$100.00"))
			Expect(USD.ToStringNoSymbol(Cents(-10000))).To(Equal("-100.00"))

			// Zero Decimal
			Expect(JPY.ToString(Cents(-10000))).To(Equal("-¥10000"))
			Expect(JPY.ToStringNoSymbol(Cents(-10000))).To(Equal("-10000"))
		})

		// Every case above is an exact multiple of 100, which is the only
		// reason they passed. The old hand-rolled rendering printed
		// cents/100, a dot, then cents%100, and Go's % keeps the sign of the
		// dividend — so a refund that was not a whole number of dollars came
		// out with a SECOND minus sign inside the fraction: -1999 rendered as
		// "-19.-99" and -1 as "0.-1". A partial refund is the most common
		// negative amount in the system.
		It("Should render negative cents that are not whole dollars", func() {
			Expect(USD.ToStringNoSymbol(Cents(-1999))).To(Equal("-19.99"))
			Expect(USD.ToStringNoSymbol(Cents(-1))).To(Equal("-0.01"))
			Expect(USD.ToStringNoSymbol(Cents(-5))).To(Equal("-0.05"))
			Expect(USD.ToStringNoSymbol(Cents(-123456))).To(Equal("-1234.56"))

			Expect(USD.ToString(Cents(-1999))).To(Equal("-$19.99"))
			Expect(USD.ToString(Cents(-1))).To(Equal("-$0.01"))
		})

		// A rendered amount must read back as the amount it was rendered
		// from. This is the law the providers depend on when they put a
		// decimal string on the wire.
		It("Should round-trip through money.ParseMinor", func() {
			for _, c := range []Cents{0, 1, -1, 99, -99, 1999, -1999, 123456, -123456} {
				s := USD.ToStringNoSymbol(c)
				back, err := money.ParseMinor(s, USD.Money())
				Expect(err).ToNot(HaveOccurred())
				Expect(Cents(back)).To(Equal(c))
			}
		})
	})

	// Type.ToFloat is gone: there is no "currency to float" concept, only the
	// last inch of a third-party API whose schema demands a JSON number. That
	// edge spells it out — Amount(c).AsMajorUnits() — so the float is visible
	// where it happens and cannot leak inward.
	Context("Type.Amount().AsMajorUnits", func() {
		It("Should render positive currency cents correctly", func() {
			// Non Zero Decimal
			Expect(USD.Amount(Cents(10023)).AsMajorUnits()).To(Equal(100.23))

			// Zero Decimal
			Expect(JPY.Amount(Cents(10023)).AsMajorUnits()).To(Equal(10023.0))
		})

		It("Should render negative currency cents correctly", func() {
			// Non Zero Decimal
			Expect(USD.Amount(Cents(-10023)).AsMajorUnits()).To(Equal(-100.23))

			// Zero Decimal
			Expect(JPY.Amount(Cents(-10023)).AsMajorUnits()).To(Equal(-10023.0))
		})
	})

	Context("Type.Label/Type.Code", func() {
		It("Should render Label and Code correctly", func() {
			Expect(USD.Label()).To(Equal("$ USD"))
			Expect(USD.Code()).To(Equal("USD"))
		})
	})

	// Crypto-Land

	Context("Type.IsCrypto", func() {
		It("Should render detect crypto-ness correctly", func() {
			Expect(USD.IsCrypto()).To(BeFalse())
			Expect(BTC.IsCrypto()).To(BeTrue())
		})
	})

	Context("Type.MinimalUnitFactor", func() {
		It("Should get conversion factor correctly", func() {
			Expect(USD.MinimalUnitFactor().Cmp(big.NewInt(1)) == 0).To(BeTrue())
			Expect(ETH.MinimalUnitFactor().Cmp(big.NewInt(1e9)) == 0).To(BeTrue())
		})
	})

	Context("Type.ToMinimalUnits", func() {
		It("Should convert correctly", func() {
			Expect(USD.ToMinimalUnits(Cents(1)).Cmp(big.NewInt(1)) == 0).To(BeTrue())
			Expect(ETH.ToMinimalUnits(Cents(1)).Cmp(big.NewInt(1e9)) == 0).To(BeTrue())
		})
	})

	Context("Type.FromMinimalUnits", func() {
		It("Should convert correctly", func() {
			Expect(USD.FromMinimalUnits(big.NewInt(1))).To(Equal(Cents(1)))
			Expect(ETH.FromMinimalUnits(big.NewInt(1e9))).To(Equal(Cents(1)))
		})

		It("Should truncate really small values", func() {
			Expect(ETH.FromMinimalUnits(big.NewInt(1e6))).To(Equal(Cents(0)))
		})
	})
})
