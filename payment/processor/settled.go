package processor

import "strings"

// Settled reports whether a gateway payment status means the funds were
// CAPTURED — money actually taken — as opposed to merely authorized.
//
// This is the ONE definition of settled. Both directions of the money path call
// it: the outbound charge path (a Charge result counts as success only when
// settled) and the inbound webhook path (a settlement event credits a balance
// only when settled). Two copies of this concept is how they drifted apart, so
// there is exactly one.
//
// COMPLETED is the terminal captured state Square reports; CAPTURED is the
// equivalent from gateways that name it that way. Comparison is
// case-insensitive because processors disagree on casing.
//
// APPROVED is deliberately NOT settled. It is an authorization hold: the funds
// are reserved, not taken, and the hold can still be voided or left to expire.
// Treating it as money mints balance against funds that may never arrive — a
// card-verification pre-auth is exactly that shape, authorized and then
// immediately voided by design. An authorization that IS later captured emits a
// further status change to COMPLETED, so refusing APPROVED delays the credit
// until capture rather than dropping it.
func Settled(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "COMPLETED", "CAPTURED":
		return true
	}
	return false
}
