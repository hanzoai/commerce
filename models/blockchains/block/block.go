package block

import (
	"hanzo.io/models/mixin"

	. "hanzo.io/models/blockchains"
)

// Datastructure for Bitcoin Block
type BitcoinBlock struct {
	BitcoinNumber string `json:"bitcoinNumber"`
}

// Datastructure for Ethereum Block
// We cannot use a named embedded struct because NodeJS client has to interact
// with this and for whatever reason, the js client cannot generate property
// names with '.' in them like the go client can.  You get an error:
//
// Error: property.name contains a path delimiter, and the entity contains one or more indexed entity value.
//
type EthereumBlock struct {
	EthereumNumber           int64     `json:"ethereumNumber"`
	EthereumHash             string    `json:"ethereumHash"`
	EthereumParentHash       string    `json:"ethereumParentHash"`
	EthereumNonce            string    `json:"ethereumNonce"`
	EthereumSha3Uncles       string    `json:"ethereumSha3Uncles"`
	EthereumLogsBloom        string    `json:"ethereumLogsBloom"`
	EthereumTransacitonsRoot string    `json:"ethereumTransactionsRoot"`
	EthereumStateRoot        string    `json:"ethereumStateRoot"`
	EthereumMiner            string    `json:"ethereumMiner"`
	EthereumDifficulty       BigNumber `json:"ethereumDifficulty"`
	EthereumTotalDifficulty  BigNumber `json:"ethereumTotalDifficulty"`
	EthereumExtraData        string    `json:"ethereumExtraData"`
	EthereumSize             int64     `json:"ethereumSize"`
	EthereumGasLimit         int64     `json:"ethereumGasLimit"`
	EthereumGasUsed          int64     `json:"ethereumGasUsed"`
	EthereumTimeStamp        int64     `json:"ethereumTimestamp"`
	EthereumUncles           []string  `json:"ethereumUncles"`
}

// Datastructure combining all the different types of blockchain blocks
type Block struct {
	mixin.Model

	BitcoinBlock
	EthereumBlock

	Type   Type          `json:"type"`
	Status ProcessStatus `json:"status"`
}
