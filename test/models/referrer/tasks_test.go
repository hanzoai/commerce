package test

import (
	"time"

	. "crowdstart.com/models/referrer"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func mkdate(year int, month time.Month, day int) func (int, int, int, int) time.Time {
	return func(hour, min, sec, nsec int) time.Time {
		return time.Date(year, month, day, hour, min, sec, nsec, time.UTC)
	}
}

var width = 10 * time.Minute

var _ = Describe("models/referrer/tasks", func() {
	Context("interval and delay calculation", func() {
		It("should calculate intervals correctly", func() {
			date := mkdate(2100, time.August, 12)
			t := date(0, 51, 5, 33)
			interval := IntervalFromInstant(t, width)
			Expect(interval.Start).To(Equal(date(0, 50, 0, 0)))
			Expect(interval.End).To(Equal(date(1, 0, 0, 0)))

			date = mkdate(2008, time.February, 29)
			t = date(23, 53, 44, 13)
			interval = IntervalFromInstant(t, width)
			Expect(interval.Start).To(Equal(date(23, 50, 0, 0)))
			Expect(interval.End).To(Equal(mkdate(2008, time.March, 1)(0, 0, 0, 0)))

			// the go stdlib time library does not support leap seconds.
			// see: https://github.com/golang/go/issues/15247
			/*
			date = mkdate(2005, 12, 31)
			t = date(23, 59, 60, 00)
			interval = IntervalFromInstant(t, width)
			Expect(interval.Start).To(Equal(date(0, 50, 0, 0)))
			Expect(interval.End).To(Equal(mkdate(2006, 1, 1)(0, 0, 0, 0)))
			*/
		})

		It("should generate unique time-related task names", func() {
			date := mkdate(2100, time.August, 12)
			t := date(0, 51, 5, 33)
			interval := IntervalFromInstant(t, width)
			label := NameFromInterval(6 * time.Minute, interval)
			Expect(label).To(Equal("processReferrals-360000000-from-2100-08-12T00:50:00Z-to-2100-08-12T01:00:00Z"))
		})
	})
})
