package types

import (
	"time"
)

type StatusCode int

const (
	Pending StatusCode = iota
	Open
	Declined
	Closed
	Completed
)

type ReasonCode int

// This is a little confusing, since Amazon conflates these reason codes
// over a number of separate uses.  There will be comments on each enum value
// denoting which use that specific entry is for.
const (
	InvalidPaymentMethod    ReasonCode = iota // Authorization - Declined
	AmazonRejected                            // Authorization - Declined && Capture - Declined
	ProcessingFailure                         // Authorization - Declined && Capture - Declined && Refund - Declined
	TransactionTimedOut                       // Authorization - Declined
	ExpiredUnused                             // Authorization - Closed
	MaxCapturesProcessed                      // Authorization - Closed
	AmazonClosed                              // Authorization - CLosed && Capture - Closed && Refund - Declined
	OrderReferenceCancelled                   // Authorization - Closed
	SellerClosed                              // Authorization - Closed
	MaxAmountRefunded                         // Capture - Closed
	MaxRefundsProcessed                       // Capture - Closed
)

type Status struct {
	State               StatusCode // They expect this to be a string going out, so stringify the enum representation.
	LastUpdateTimeStamp time.Time  // ISO 8601 format
	ReasonCode          ReasonCode // They expect this as a string going out, so stringify the enum representation if needed.
	ReasonDescription   string     // Optional description of the status.  max length: 255 characters
}
