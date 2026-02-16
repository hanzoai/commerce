package promotionrule

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"
)

type PromotionRule struct {
	mixin.Model

	PromotionId string `json:"promotionId"`
	Attribute   string `json:"attribute"`
	Operator    string `json:"operator"`

	// Values stored as JSON-encoded string in datastore
	Values  []string `json:"values" datastore:"-"`
	Values_ string   `json:"-" datastore:",noindex"`
}

func (p *PromotionRule) Load(ps []datastore.Property) (err error) {
	p.Defaults()

	if err = datastore.LoadStruct(p, ps); err != nil {
		return err
	}

	if len(p.Values_) > 0 {
		err = json.DecodeBytes([]byte(p.Values_), &p.Values)
	}

	return err
}

func (p *PromotionRule) Save() ([]datastore.Property, error) {
	p.Values_ = string(json.EncodeBytes(&p.Values))

	return datastore.SaveStruct(p)
}
