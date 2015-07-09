package types

// At least one of these must be specified.
type SellerBillingAgreementAttributes struct {
	SellerBillingAgreementId string // Merchant-specified identifier for the agreement.
	StoreName                string // The store the order was placed from.
	CustomInformation        string // Any extra misc information.
}
