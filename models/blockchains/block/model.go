package block

import (
	"github.com/hanzoai/commerce/datastore"
	. "github.com/hanzoai/commerce/models/blockchains"
)

var kind = "block"

func (b Block) Kind() string {
	return kind
}

func (b *Block) Init(db *datastore.Datastore) {
	b.BaseModel.Init(db, b)
}

func (b *Block) Defaults() {
}

func New(db *datastore.Datastore) *Block {
	b := new(Block)
	nsDb := datastore.New(db.Context)
	nsDb.SetNamespace(BlockchainNamespace)
	b.Init(nsDb)
	b.Defaults()
	return b
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
