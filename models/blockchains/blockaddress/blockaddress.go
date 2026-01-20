package blockaddress

import (
	"github.com/hanzoai/commerce/models/mixin"

	. "github.com/hanzoai/commerce/models/blockchains"
)

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
	mixin.Model

	WatchedAddress

	// Address on the blockchain
	Address string `json:"address"`

	// Which blockchain contains the address
	Type Type `json:"type"`
}
