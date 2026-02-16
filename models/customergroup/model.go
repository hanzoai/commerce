package customergroup

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var kind = "customergroup"

func (g CustomerGroup) Kind() string {
	return kind
}

func (g *CustomerGroup) Init(db *datastore.Datastore) {
	g.Model.Init(db, g)
}

func (g *CustomerGroup) Defaults() {
	g.Metadata = make(Map)
}

func New(db *datastore.Datastore) *CustomerGroup {
	g := new(CustomerGroup)
	g.Init(db)
	g.Defaults()
	return g
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
