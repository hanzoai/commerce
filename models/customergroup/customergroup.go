package customergroup

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/orm"

	. "github.com/hanzoai/commerce/types"
)

func init() { orm.Register[CustomerGroup]("customergroup") }

type CustomerGroup struct {
	mixin.EntityBridge[CustomerGroup]

	Name string `json:"name"`

	// Arbitrary metadata
	Metadata  Map    `json:"metadata,omitempty" datastore:"-" orm:"default:{}"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (g *CustomerGroup) Load(ps []datastore.Property) (err error) {
	if err = datastore.LoadStruct(g, ps); err != nil {
		return err
	}

	if len(g.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(g.Metadata_), &g.Metadata)
	}

	return err
}

func (g *CustomerGroup) Save() ([]datastore.Property, error) {
	g.Metadata_ = string(json.EncodeBytes(&g.Metadata))

	return datastore.SaveStruct(g)
}

func New(db *datastore.Datastore) *CustomerGroup {
	g := new(CustomerGroup)
	g.Init(db)
	return g
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("customergroup")
}
