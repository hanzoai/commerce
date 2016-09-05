package test

import (
	"time"
	"testing"

	"crowdstart.com/models/affiliate"
	"crowdstart.com/periodic/affiliate_transfer"

	. "crowdstart.com/util/test/ginkgo"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "periodic/affiliate_transfer/job")
}

var _ = Describe("periodic/affiliate_transfer/job", func() {
	It("should calculate payment time delays correctly", func() {
		var aff affiliate.Affiliate

		aff.Period = 12
		orig := time.Date(2100, time.August, 12, 10, 5, 5, 5, time.UTC)
		cutoff := affiliate_transfer.CutoffForAffiliate(aff, orig)
		expected := time.Date(2100, time.July, 31, 0, 0, 0, 0, time.UTC)
		Expect(cutoff).To(Equal(expected))

		aff.Period = 5
		orig = time.Date(1990, time.January, 2, 1, 2, 3, 4, time.UTC)
		cutoff = affiliate_transfer.CutoffForAffiliate(aff, orig)
		expected = time.Date(1989, time.December, 28, 0, 0, 0, 0, time.UTC)
		Expect(cutoff).To(Equal(expected))

		aff.Period = 1
		orig = time.Date(2016, time.March, 1, 15, 29, 33, 20, time.UTC)
		cutoff = affiliate_transfer.CutoffForAffiliate(aff, orig)
		expected = time.Date(2016, time.February, 29, 0, 0, 0, 0, time.UTC)
		Expect(cutoff).To(Equal(expected))

		aff.Period = 1
		orig = time.Date(2015, time.March, 1, 15, 29, 33, 20, time.UTC)
		cutoff = affiliate_transfer.CutoffForAffiliate(aff, orig)
		expected = time.Date(2015, time.February, 28, 0, 0, 0, 0, time.UTC)
		Expect(cutoff).To(Equal(expected))
	})
})
