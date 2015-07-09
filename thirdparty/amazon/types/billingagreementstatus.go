package types

import "time"

type BillingAgreementStatus struct {
	State                string    // The current state of the billing agreement object
	LastUpdatedTimeStamp time.Time // When it was last updated
	ReasonCode           string    // Might be filled out if billing agreement is in DRAFT
	ReasonDescription    string    // Optional description blah blah

}
