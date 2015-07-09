package types

import (
	"time"
)

type BillingAgreementDetails struct {
	AmazonBillingAgreementId         string                           // Retrieved from the Button, AddressBook, or Wallet widgets
	BillingAgreementLimits           BillingAgreementLimits           // Total amount we can charge a buyer during a given time period.
	Buyer                            Buyer                            // Details on the buyer
	SellerNote                       string                           // Represents a description of the billing agreement displayed in emails to the buyer.
	PlatformId                       string                           // Our SellerId (crowdstart's)
	Destination                      Destination                      // The address selected by the buyer.
	ReleaseEnvironment               string                           // Live or Sandbox
	SellerBillingAgreementAttributes SellerBillingAgreementAttributes // Context about the billing agreement
	BillingAgreementStatus           BillingAgreementStatus           // Current status of the billing agreement.
	Constraints                      []Constraint                     // A list of things that are missing or incorrect (so hopefully this is null)
	CreationTimestamp                time.Time                        // ISO 8601
	BillingAgreementConsent          bool                             // Indicates buyer's consent to the agreement.
}
