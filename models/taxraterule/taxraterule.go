package taxraterule

import (
	"github.com/hanzoai/commerce/models/mixin"
)

type TaxRateRule struct {
	mixin.Model

	TaxRateId   string `json:"taxRateId"`
	Reference   string `json:"reference"`
	ReferenceId string `json:"referenceId"`
}
