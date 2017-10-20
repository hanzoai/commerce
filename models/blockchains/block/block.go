package block

import (
	"hanzo.io/models/mixin"

	. "hanzo.io/models/blockchains"
)

// Datastructure for Bitcoin Block
type BitcoinBlock struct {
	BitcoinBlockNumber string `json:"bitcoinBlockNumber"`
}

// Datastructure for Ethereum Block
// We cannot use a named embedded struct because NodeJS client has to interact
// with this and for whatever reason, the js client cannot generate property
// names with '.' in them like the go client can.  You get an error:
//
// Error: property.name contains a path delimiter, and the entity contains one or more indexed entity value.
//
type EthereumBlock struct {
	EthereumBlockNumber           int64     `json:"ethereumBlockNumber"`
	EthereumBlockHash             string    `json:"ethereumBlockHash"`
	EthereumBlockParentHash       string    `json:"ethereumBlockParentHash"`
	EthereumBlockNonce            string    `json:"ethereumBlockNonce"`
	EthereumBlockSha3Uncles       string    `json:"ethereumBlockSha3Uncles"`
	EthereumBlockLogsBloom        string    `json:"ethereumBlockLogsBloom"`
	EthereumBlockTransacitonsRoot string    `json:"ethereumBlockTransactionsRoot"`
	EthereumBlockStateRoot        string    `json:"ethereumBlockStateRoot"`
	EthereumBlockMiner            string    `json:"ethereumBlockMiner"`
	EthereumBlockDifficulty       BigNumber `json:"ethereumBlockDifficulty"`
	EthereumBlockTotalDifficulty  BigNumber `json:"ethereumBlockTotalDifficulty"`
	EthereumBlockExtraData        string    `json:"ethereumBlockExtraData"`
	EthereumBlockSize             int64     `json:"ethereumBlockSize"`
	EthereumBlockGasLimit         int64     `json:"ethereumBlockGasLimit"`
	EthereumBlockGasUsed          int64     `json:"ethereumBlockGasUsed"`
	EthereumBlockTimeStamp        int64     `json:"ethereumBlockTimestamp"`
	EthereumBlockUncles           []string  `json:"ethereumBlockUncles"`
}

// Datastructure combining all the different types of blockchain blocks
type Block struct {
	mixin.Model

	BitcoinBlock
	EthereumBlock

	Type Type `json:"type"`
	// Status        ProcessStatus `json:"status"`
	Confirmations int64 `json:"confirmations"`
}
