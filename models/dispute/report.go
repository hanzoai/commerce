package dispute

import (
	"strings"
	"time"
)

// StateWon is the processor's dispute state in which the merchant kept the
// payment: the withheld funds were returned.
const StateWon = "WON"

// Withholds reports whether the processor withholds a disputed payment's funds in
// state: a chargeback (EVIDENCE_REQUIRED, PROCESSING), or one lost or accepted. An
// inquiry (INQUIRY_EVIDENCE_REQUIRED, INQUIRY_PROCESSING, INQUIRY_CLOSED) holds no
// funds, and a dispute won has had them returned.
func Withholds(state string) bool {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "EVIDENCE_REQUIRED", "PROCESSING", "LOST", "ACCEPTED":
		return true
	}
	return false
}

// phase is how far along a dispute in state is: an inquiry, a chargeback, or
// decided. A dispute only moves forward through them, so a report from an earlier
// phase than the one held is the older of the two.
func phase(state string) int {
	switch state {
	case "INQUIRY_EVIDENCE_REQUIRED", "INQUIRY_PROCESSING", "INQUIRY_CLOSED":
		return 1
	case "EVIDENCE_REQUIRED", "PROCESSING":
		return 2
	case StateWon, "LOST", "ACCEPTED":
		return 3
	}
	return 0
}

// Report is the latest state a processor reported for a dispute of one payment,
// with the dispute's version and updated_at that order reports against each other.
type Report struct {
	Dispute        string    `json:"dispute,omitempty"`
	DisputeVersion int64     `json:"disputeVersion,omitempty"`
	DisputeAt      time.Time `json:"disputeAt,omitempty"`
}

// older reports whether a report of state at version and updated_at at is older
// than the held one: by the dispute's version when both carry one, else by its
// updated_at when both carry one, else by the dispute's phase.
func (r *Report) older(state string, version int64, at time.Time) bool {
	switch {
	case version > 0 && r.DisputeVersion > 0:
		return version < r.DisputeVersion
	case !at.IsZero() && !r.DisputeAt.IsZero():
		return at.Before(r.DisputeAt)
	}
	return phase(state) > 0 && phase(state) < phase(r.Dispute)
}

// SetDispute records a reported state, in upper case, and reports whether it is
// the latest known: a report older than the one held, or one other than WON after
// a dispute was won, changes nothing and answers false. The same report again
// answers true. A version or updated_at the report does not carry keeps the held
// one.
func (r *Report) SetDispute(state string, version int64, at time.Time) bool {
	state = strings.ToUpper(strings.TrimSpace(state))
	if r.older(state, version, at) {
		return false
	}
	if r.Dispute == StateWon && state != StateWon {
		return false
	}
	r.Dispute = state
	if version > 0 {
		r.DisputeVersion = version
	}
	if !at.IsZero() {
		r.DisputeAt = at
	}
	return true
}

// Disputed reports whether the reported dispute withholds the payment's funds.
func (r Report) Disputed() bool {
	return Withholds(r.Dispute)
}
