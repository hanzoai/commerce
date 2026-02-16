package taxprovider

import (
	"github.com/hanzoai/commerce/models/mixin"
)

type TaxProvider struct {
	mixin.Model

	Name      string `json:"name"`
	IsEnabled bool   `json:"isEnabled"`
}
