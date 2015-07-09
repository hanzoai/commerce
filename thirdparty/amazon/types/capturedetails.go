package types

import "time"

type CaptureDetails struct {
	AmazonCaptureId    string    // Amazon's generated ID for this capture
	CaptureReferenceId string    // Our generated ID for this capture.  Max length: 32 chars
	CaptureAmount      Price     // Amount to be captured.
	RefundedAmount     Price     // Total amount refunded on this capture.
	CaptureFee         Price     // Amazon's cut.
	IdList             []string  // A list of AmazonRefundId identifiers requested on the capture object.
	CreationTimestamp  time.Time // ISO 8601
	CaptureStatus      Status    // Status of the capture
	SoftDescriptor     string    // Description to be shown on the buyer's payment statement.  Max length: 16 chars
}
