package variantinventorylink

import (
	"github.com/hanzoai/commerce/models/mixin"
)

type VariantInventoryLink struct {
	mixin.BaseModel

	VariantId       string `json:"variantId"`
	InventoryItemId string `json:"inventoryItemId"`
}
