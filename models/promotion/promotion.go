package promotion

import (
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

type Promotion struct {
	mixin.Model

	Code           string     `json:"code"`
	Type           string     `json:"type"`
	Status         string     `json:"status"`
	IsAutomatic    bool       `json:"isAutomatic"`
	IsTaxInclusive bool       `json:"isTaxInclusive"`
	CampaignId     string     `json:"campaignId"`
	StartsAt       *time.Time `json:"startsAt,omitempty"`
	EndsAt         *time.Time `json:"endsAt,omitempty"`

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (p *Promotion) Load(ps []datastore.Property) (err error) {
	p.Defaults()

	if err = datastore.LoadStruct(p, ps); err != nil {
		return err
	}

	if len(p.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(p.Metadata_), &p.Metadata)
	}

	return err
}

func (p *Promotion) Save() ([]datastore.Property, error) {
	p.Metadata_ = string(json.EncodeBytes(&p.Metadata))

	return datastore.SaveStruct(p)
}
