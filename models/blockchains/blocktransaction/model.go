package blocktransaction

import (
	"appengine"

	"hanzo.io/datastore"
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
	if err, ctx := appengine.Namespace(db.Context, "blockchains"); err != nil {
		panic(err)
	} else {
		b.Init(datastore.New(ctx))
	}
	b.Defaults()
	return b
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
