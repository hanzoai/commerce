package blockaddress

import (
	"github.com/hanzoai/commerce/datastore"
	. "github.com/hanzoai/commerce/models/blockchains"
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
	nsDb := datastore.New(db.Context)
	nsDb.SetNamespace(BlockchainNamespace)
	b.Init(nsDb)
	b.Defaults()
	return b
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
