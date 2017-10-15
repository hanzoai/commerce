package block

import (
	"appengine"

	"hanzo.io/datastore"
)

var kind = "block"

func (b Block) Kind() string {
	return kind
}

func (b *Block) Init(db *datastore.Datastore) {
	b.Model.Init(db, b)
}

func (b *Block) Defaults() {
}

func New(db *datastore.Datastore) *Block {
	b := new(Block)
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
