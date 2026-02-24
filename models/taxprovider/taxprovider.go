package taxprovider

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/orm"
)

func init() { orm.Register[TaxProvider]("taxprovider") }

type TaxProvider struct {
	mixin.Model[TaxProvider]

	Name      string `json:"name"`
	IsEnabled bool   `json:"isEnabled" orm:"default:true"`
}

func New(db *datastore.Datastore) *TaxProvider {
	t := new(TaxProvider)
	t.Init(db)
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("taxprovider")
}
