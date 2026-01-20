package blocktransaction

import (
	"github.com/hanzoai/commerce/datastore"
	. "github.com/hanzoai/commerce/models/blockchains"
)

var kind = "blocktransaction"

func (b BlockTransaction) Kind() string {
	return kind
}

func (b *BlockTransaction) Init(db *datastore.Datastore) {
	b.Model.Init(db, b)
}

func (b *BlockTransaction) Defaults() {
}

func New(db *datastore.Datastore) *BlockTransaction {
	b := new(BlockTransaction)
	nsDb := datastore.New(db.Context)
	nsDb.SetNamespace(BlockchainNamespace)
	b.Init(nsDb)
	b.Defaults()
	return b
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
