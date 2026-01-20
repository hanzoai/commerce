package block

import (
	"google.golang.org/appengine"

	"github.com/hanzoai/commerce/datastore"
	. "github.com/hanzoai/commerce/models/blockchains"
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
	if ctx, err := appengine.Namespace(db.Context, BlockchainNamespace); err != nil {
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
