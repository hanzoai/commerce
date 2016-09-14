package test

import (
	"time"

	"crowdstart.com/util/log"

	. "crowdstart.com/models/types/schedule"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	start = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
)

// Net D should be supported
// Jan 1 2000
// Feb 1 2000
// Mar 1 2000
// 	Jan -> Feb
// Mar 2 2000
// 	Jan -> Feb
// Mar 3 2000
// 	Jan -> Feb
// Apri 1
// 	Jan -> Mar

var _ = Describe("models/types/schedule", func() {
	Context("Schedule.Started()", func() {
		It("should know whether we've started yet", func() {
			s := Schedule{}
			Expect(s.Started()).To(Equal(false))
			s.StartAt = time.Now()
			Expect(s.Started()).To(Equal(true))
		})
	})

	Context("Schedule.Cutoff()", func() {
		It("should calculate cut-off", func() {
			s := Schedule{}
			s.Period = 30
			log.Warn("Cutoff: %v", s.Cutoff())
		})
	})
})
