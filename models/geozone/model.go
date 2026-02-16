package geozone

import (
	"github.com/hanzoai/commerce/datastore"

	. "github.com/hanzoai/commerce/types"
)

var kind = "geozone"

func (g GeoZone) Kind() string {
	return kind
}

func (g *GeoZone) Init(db *datastore.Datastore) {
	g.Model.Init(db, g)
}

func (g *GeoZone) Defaults() {
	g.Metadata = make(Map)
}

func New(db *datastore.Datastore) *GeoZone {
	g := new(GeoZone)
	g.Init(db)
	g.Defaults()
	return g
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
