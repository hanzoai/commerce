package types

import "time"

type OrderReferenceStatus struct {
	State               string    // The state the order is in, Draft/Open/Suspended/Closed/Canceled
	LastUpdateTimestamp time.Time // ISO 8601
	ReasonCode          string    // Reason the state the order is in the state it's in, mostly used for fail states.
	ReasonDescription   string    // Optional extra description.
}
