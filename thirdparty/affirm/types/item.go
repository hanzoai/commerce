package types

type Item struct {
	Sku          string `json:"sku"`            // The item sku
	Qty          int    `json:"qty,omitempty"`  // The quantity, defaults to 1
	UnitPrice    int    `json:"unit_price"`     // The unit price
	DisplayName  string `json:"display_name"`   // THe label displayed to the customer
	ItemUrl      string `json:"item_url"`       // The item product page url
	ItemImageUrl string `json:"item_image_url"` // The item image url
}
