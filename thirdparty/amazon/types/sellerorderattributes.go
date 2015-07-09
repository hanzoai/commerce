package types

type SellerOrderAttributes struct {
	SellerOrderId     string // Merchant-specified identifier of this order
	StoreName         string // Store the order was placed from
	CustomInformation string // Any additional information we wnat to include.
}
