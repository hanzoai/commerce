package blocktransaction

import (
	"hanzo.io/models/mixin"

	. "hanzo.io/models/blockchains"
)

// Datastructure for Bitcoin Transaction
type BitcoinTransaction struct {
}

// Datastructure for Ethereum Transaction
type EthereumTransaction struct {
	EthereumHash             string    `json:"ethereumHash"`
	EthereumNonce            int64     `json:"ethereumNonce"`
	EthereumBlockHash        string    `json:"ethereumBlockHash"`
	EthereumBlockNumber      int64     `json:"ethereumBlockNumber"`
	EthereumTransactionIndex int64     `json:"ethereumTransactionIndex"`
	EthereumFrom             string    `json:"ethereumFrom"`
	EthereumTo               string    `json:"ethereumTo"`
	EthereumValue            BigNumber `json:"ethereumValue"`
	EthereumGasPrice         BigNumber `json:"ethereumGasPrice"`
	EthereumGas              BigNumber `json:"ethereumGas"`
	EthereumInput            string    `json:"ethereumInput"`
}

// Datastructure for Bitcoin Transaction Receipt
type BitcoinTransactionReceipt struct {
}

// Figure out how to support this later
// Datastructure for Ethereum Transaction Logs
// type EthereumTransactionLog struct {
// 	LogIndex         int64  `json:"logIndex"`
// 	BlockHash        string `json:"blockHash"`
// 	BlockNumber      int64  `json:"blockNumber"`
// 	TransactionHash  string `json:"transactionHash"`
// 	TransactionIndex int64  `json:"transactionIndex"`
// 	Address          string `json:"address"`
// 	Data             string `json:"data"`
// 	Topics           string `json:"topics"`
// }

// Datastructure for Ethereum Transaction Receipt
type EthereumTransactionReceipt struct {
	EthereumBlockHash         string    `json:"ethereumBlockHash"`
	EthereumBlockNumber       int64     `json:"ethereumBlockNumber"`
	EthereumTransactionHash   string    `json:"ethereumTransactionHash"`
	EthereumTransactionIndex  int64     `json:"ethereumTransactionIndex"`
	EthereumFrom              string    `json:"ethereumFrom"`
	EthereumTo                string    `json:"ethereumTo"`
	EthereumCumulativeGasUsed BigNumber `json:"ethereumCumulativeGasUsed"`
	EthereumGasUsed           BigNumber `json:"ethereumGasUsed"`
	EthereumContractAddress   string    `json:"ethereumContractAddress"`
	// Logs              []EthereumTransactionLog `json:"logs,omitempty"`
}

// Datastructure combining all the different types of transactions
type BlockTransaction struct {
	mixin.Model

	Address string `json:"address"`

	BitcoinTransaction
	EthereumTransaction
	EthereumTransactionReceipt

	Type   Type          `json:"type"`
	Status ProcessStatus `json:"status"`
	Usage  Usage         `json:"usage"`
}
