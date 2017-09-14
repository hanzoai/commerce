package test

import (
	"testing"

	"hanzo.io/models/taxrates"
	"hanzo.io/models/types/georate"
	"hanzo.io/util/fake"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("models/taxrates", t)
}

var _ = Describe("models/taxrates", func() {
	Context("Match", func() {
		It("Should Match Match with Highest Level", func() {
			grs := taxrates.TaxRates{
				GeoRates: []taxrates.GeoRate{
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "mo", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "emporia", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "", "66212", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("", "", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
				},
			}

			gr, level, idx := grs.Match("us", "ks", "", "66212")
			Expect(gr).To(Equal(&grs.GeoRates[3]))
			Expect(level).To(Equal(3))
			Expect(idx).To(Equal(3))
		})

		It("Should Return L0 Default Rates", func() {
			grs := taxrates.TaxRates{
				GeoRates: []taxrates.GeoRate{
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "mo", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("", "", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "emporia", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "", "66212", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
				},
			}

			gr, level, idx := grs.Match("gb", "bkm", "", "sl8")
			Expect(gr).To(Equal(&grs.GeoRates[2]))
			Expect(level).To(Equal(0))
			Expect(idx).To(Equal(2))
		})

		It("Should Return L1 Country Rates", func() {
			grs := taxrates.TaxRates{
				GeoRates: []taxrates.GeoRate{
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "mo", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("", "", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "emporia", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "", "66212", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
				},
			}

			gr, level, idx := grs.Match("US", "ky", "", "12345")
			Expect(gr).To(Equal(&grs.GeoRates[5]))
			Expect(level).To(Equal(1))
			Expect(idx).To(Equal(5))
		})

		It("Should Return L2 State Rates", func() {
			grs := taxrates.TaxRates{
				GeoRates: []taxrates.GeoRate{
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "mo", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("", "", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "emporia", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "", "66212", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
				},
			}

			gr, level, idx := grs.Match("us", "kS", "", "12345")
			Expect(gr).To(Equal(&grs.GeoRates[0]))
			Expect(level).To(Equal(2))
			Expect(idx).To(Equal(0))
		})

		It("Should Return L3 City Rates", func() {
			grs := taxrates.TaxRates{
				GeoRates: []taxrates.GeoRate{
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "mo", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("", "", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "emporia", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "", "66212", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
				},
			}

			gr, level, idx := grs.Match("uS", "Ks", "Emporia", "")
			Expect(gr).To(Equal(&grs.GeoRates[3]))
			Expect(level).To(Equal(3))
			Expect(idx).To(Equal(3))
		})

		It("Should Return L3 Postal Code Rates", func() {
			grs := taxrates.TaxRates{
				GeoRates: []taxrates.GeoRate{
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "mo", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("", "", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "emporia", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "", "66212", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
				},
			}

			gr, level, idx := grs.Match("us", "ks", "", "66212")
			Expect(gr).To(Equal(&grs.GeoRates[4]))
			Expect(level).To(Equal(3))
			Expect(idx).To(Equal(4))
		})

		It("Should Fail Without a Default Rate", func() {
			grs := taxrates.TaxRates{
				GeoRates: []taxrates.GeoRate{
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "mo", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "emporia", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "ks", "", "66212", 0.1, 1),
						TaxName: fake.Characters(16),
					},
					taxrates.GeoRate{
						GeoRate: georate.New("us", "", "", "", 0.1, 1),
						TaxName: fake.Characters(16),
					},
				},
			}

			gr, level, idx := grs.Match("gb", "bkm", "", "sl8")
			Expect(gr).To(BeNil())
			Expect(level).To(Equal(-1))
			Expect(idx).To(Equal(-1))
		})
	})
})
