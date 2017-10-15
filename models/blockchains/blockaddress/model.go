package blockaddress

import (
	"hanzo.io/datastore"
)

var kind = "blockaddress"

func (b BlockAddress) Kind() string {
	return kind
}

func (b *BlockAddress) Init(db *datastore.Datastore) {
	b.Model.Init(db, b)
}

func (b *BlockAddress) Defaults() {
}

func New(db *datastore.Datastore) *BlockAddress {
	b := new(BlockAddress)
	b.Init(db)
	b.Defaults()
	return b
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
