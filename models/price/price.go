package price

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/orm"

	. "github.com/hanzoai/commerce/types"
)

func init() { orm.Register[Price]("price") }

type Price struct {
	mixin.EntityBridge[Price]

	PriceSetId   string         `json:"priceSetId"`
	CurrencyCode string         `json:"currencyCode"`
	Amount       currency.Cents `json:"amount"`
	MinQuantity  int            `json:"minQuantity"`
	MaxQuantity  int            `json:"maxQuantity"`
	PriceListId  string         `json:"priceListId"`

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (p *Price) Load(ps []datastore.Property) (err error) {
	if err = datastore.LoadStruct(p, ps); err != nil {
		return err
	}

	if len(p.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(p.Metadata_), &p.Metadata)
	}

	return err
}

func (p *Price) Save() ([]datastore.Property, error) {
	p.Metadata_ = string(json.EncodeBytes(&p.Metadata))

	return datastore.SaveStruct(p)
}

func New(db *datastore.Datastore) *Price {
	t := new(Price)
	t.Init(db)
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("price")
}
