package amazon

type SellerOrderAttributes struct {
	SellerOrderId     string //merchant-specified identifier of this order
	StoreName         string //store the order was placed from
	CustomInformation string //Any additional information we wnat to include.
}
