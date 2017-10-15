package blockaddress

import (
	"hanzo.io/models/mixin"

	. "hanzo.io/models/blockchains"
)

// This is a reference to an address the blockchain reader needs to watch for
type WatchedAddress struct {
	// This is the namespace where the walletbearer is stored
	WalletBearerNamespace string `json:"walletBearerNamespace`

	// This is the kind where the walletbearer is stored
	WalletBearerKind string `json:"walletBearerKind"`

	// This is the id of the walletbearer
	WalletBearerId string `json:"walletBearerId"`
}

// BlockAddress denotes an address on the blockchain that we wnat ot keep track
// of
type BlockAddress struct {
	mixin.Model

	Address
	WatchedAddress
}
