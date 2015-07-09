package types

type OrderReferenceAttributes struct {
	OrderTotal            OrderTotal            // Required - specifies the total amount of the order represented by this order reference
	PlatformId            string                // Crowdstart's sellerid
	SellerNote            string                // Max length: 1024 characters.  Description of the order to the buyer.
	SellerOrderAttributes SellerOrderAttributes // Context about the order.
}
