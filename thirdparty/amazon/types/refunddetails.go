package types

import "time"

type RefundDetails struct {
	AmazonRefundId    string    // Amazon generated identifier for this refund.
	RefundReferenceId string    // Merchant-generated identifier for this refund.
	SellerRefundNote  string    // A buyer-facing description for the refund.
	RefundType        string    // Always SellerInitiated
	RefundAmount      Price     // How much is being refunded
	FeeRefunded       Price     // How much amazon is returning in capture fees
	CreationTimestamp time.Time // ISO 8601
	RefundStatus      Status    // Status of the refund request
	SoftDescriptor    string    // Description shown on the buyer's payment instrument.  Max length: 16 chars
}
