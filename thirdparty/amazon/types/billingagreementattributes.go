package types

// One of the following must be specified.
type BillingAgreementAttributes struct {
	PlatformId                       string                           // Our platform ID
	SellerNote                       string                           // Max length: 1024 characters.  A description of the billing agreements sent to the buyer.
	SellerBillingAgreementAttributes SellerBillingAgreementAttributes // Context about the billing agreement
}
