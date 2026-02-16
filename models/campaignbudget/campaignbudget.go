package campaignbudget

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

type CampaignBudget struct {
	mixin.Model

	CampaignId   string `json:"campaignId"`
	Type         string `json:"type"`
	CurrencyCode string `json:"currencyCode"`
	Limit        int    `json:"limit"`
	Used         int    `json:"used"`

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (c *CampaignBudget) Load(ps []datastore.Property) (err error) {
	c.Defaults()

	if err = datastore.LoadStruct(c, ps); err != nil {
		return err
	}

	if len(c.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(c.Metadata_), &c.Metadata)
	}

	return err
}

func (c *CampaignBudget) Save() ([]datastore.Property, error) {
	c.Metadata_ = string(json.EncodeBytes(&c.Metadata))

	return datastore.SaveStruct(c)
}
