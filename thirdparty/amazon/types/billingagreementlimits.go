package types

import "time"

type BillingAgreementLimits struct {
	AmountLimitPerTimePeriod Price     // Maximum amount that can be charged on a billing agreement time period, defined by the start/end dates on this object.
	TimePeriodStartDate      time.Time // Start date during which the Amount limit applies.
	TimePeriodEndDate        time.Time // End date for which the Amount limit applies.
	CurrentRemainingBalance  Price     // Remaining balance for the time period.
}
