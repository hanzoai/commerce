package block

import (
	"hanzo.io/models/mixin"

	. "hanzo.io/models/blockchains"
)

// Datastructure for Bitcoin Block
type BitcoinBlock struct {
	BlockNumber string `json:"blockNumber"`
}

// Datastructure for Ethereum Block
type EthereumBlock struct {
	BlockNumber string `json:"blockNumber"`
}

// Datastructure combining all the different types of blockchain blocks
type Block struct {
	mixin.Model

	BitcoinBlock  BitcoinBlock  `json:"bitcoinBlock"`
	EthereumBlock EthereumBlock `json:"ethereumBlock"`

	Type   Type          `json:"type"`
	Status ProcessStatus `json:"status"`
}
