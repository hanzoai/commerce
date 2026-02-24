package blockaddress

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/orm"

	. "github.com/hanzoai/commerce/models/blockchains"
)

func init() { orm.Register[BlockAddress]("blockaddress") }

// This is a reference to an address the blockchain reader needs to watch for
type WatchedAddress struct {
	// This is the namespace where the wallet is stored
	WalletNamespace string `json:"walletNamespace`

	// This is the id of the wallet
	WalletId string `json:"walletId"`
}

// BlockAddress denotes an address on the blockchain that we wnat ot keep track
// of
type BlockAddress struct {
	mixin.Model[BlockAddress]

	WatchedAddress

	// Address on the blockchain
	Address string `json:"address"`

	// Which blockchain contains the address
	Type Type `json:"type"`
}

func New(db *datastore.Datastore) *BlockAddress {
	b := new(BlockAddress)
	nsDb := datastore.New(db.Context)
	nsDb.SetNamespace(BlockchainNamespace)
	b.Init(nsDb)
	return b
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("blockaddress")
}
