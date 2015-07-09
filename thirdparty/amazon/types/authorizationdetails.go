package types

import (
	"time"
)

type AuthorizationDetails struct {
	AmazonAuthorizationId    string    // Amazon-generated auth id for a transaction
	AuthorizationReferenceId string    // Crowdstart-generated auth.  Max length: 32 chars
	SellerAuthorizationNote  string    // Seller-facing description
	AuthorizationAmount      Price     // Amount being authed
	CapturedAmount           Price     // Amount that has been captured
	AuthorizationFee         Price     // Amazon's fee
	IdList                   []string  // A list of AmazonCaptureId identifiers that are requesting captures.  Can be empty.
	CreationTimestamp        time.Time // Time the authorization was created, ISO 8601 format
	ExpirationTimestamp      time.Time // Time when the authorization expires and you can't run a capture on it. ISO 8601.
	AuthorizationStatus      Status    // Current status of the auth, always returns PENDING in async mode
	CaptureNow               bool      // Capture right now y/n
	SoftDescriptor           string    // Payment description if CaptureNow is true.  max 16 chars
}
