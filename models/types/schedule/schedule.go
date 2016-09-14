package schedule

import (
	"time"

	"crowdstart.com/util/timeutil"
)

type Schedule struct {
	Period  int       `json:"period"`
	StartAt time.Time `json:"startAt,omitempty"`
	LastAt  time.Time `json:"lastAt,omitempty"`
	Rolling bool      `json:"rolling,omitempty"`
}

func (s Schedule) Started() bool {
	if timeutil.IsZero(s.StartAt) {
		return false
	}
	return true
}

func (s Schedule) Cutoff() time.Time {
	if s.Rolling {
		return time.Now().UTC().AddDate(0, 0, -s.Period)
	}

	// FIXME: what
	last := s.LastAt
	if timeutil.IsZero(last) {
		last = s.StartAt
	}

	past := last.UTC().AddDate(0, 0, -s.Period)
	return past
}
