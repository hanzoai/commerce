package types

import "time"

type OrderReferenceDetails struct {
	AmazonOrderReferenceId string                // Order reference identifier from Amazon Button widget
	Buyer                  Buyer                 // Details about the buyer.
	OrderTotal             OrderTotal            // Total amount for this order reference object.
	SellerNote             string                // Description of the order to the seller. Max length: 1024 chars
	PlatformId             string                // Crowdstart's sellerid
	Destination            Destination           // Address selected by the buyer.
	ReleaseEnvironment     string                //  Live or Sandbox
	SellerOrderAttributes  SellerOrderAttributes // Context about the order represented here.
	OrderReferenceStatus   OrderReferenceStatus  // Current status of the order reference.
	Constraints            []Constraint          // A list of constraints denoting information that's missing or incorrect.
	CreationTimestamp      time.Time             // ISO 8601
	ExpirationTimestamp    time.Time             // ISO 8601
	IdList                 []string              // AmazonAuthorizationId identifiers that have been requested
}
