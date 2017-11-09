package blocktransaction

import (
	"hanzo.io/models/mixin"

	. "hanzo.io/models/blockchains"
)

// Datastructure for Bitcoin Transaction
type BitcoinTransaction struct {
	BitcoinTransactionHeight        string                 `json:"bitcoinTransactionHeight"`
	BitcoinTransactionHash          string                 `json:"bitcoinTransactionHash"`
	BitcoinTransactionVersion       int64                  `json:"bitcoinTransactionVersion"`
	BitcoinTransactionSize          int64                  `json:"bitcoinTransactionSize"`
	BitcoinTransactionVSize         int64                  `json:"bitcoinTransactionVSize"`
	BitcoinTransactionLocktime      int64                  `json:"bitcoinTransactionLocktime"`
	BitcoinTransactionHex           string                 `json:"bitcoinTransactionHex"`
	BitcoinTransactionBlockHash     string                 `json:"bitcoinTransactionBlockHash"`
	BitcoinTransactionConfirmations int64                  `json:"bitcoinTransactionConfirmations"`
	BitcoinTransactionTime          int64                  `json:"bitcoinTransactionTime"`
	BitcoinTransactionBlockTime     int64                  `json:"bitcoinTransactionBlockTime"`
	BitcoinTransactionType          BitcoinTransactionType `json:"bitcoinTransactionType"`
}

type BitcoinVIn struct {
	BitcoinTransactionVInTransactionHash  string `json:"bitcoinTransactionVInTransactionHash"`
	BitcoinTransactionVInTransacitonIndex string `json:"bitcoinTransactionVInTransactionIndex"`
	BitcoinTransactionVInIndex            string `json:"bitcoinTransactionVInIndex"`
	BitcoinTransactionVInFrom             string `json:"bitcoinTransactionVInFrom"`
	BitcoinTransactionVInValue            int64  `json:"bitcoinTransactionVInValue"`
}

type BitcoinVOut struct {
	BitcoinTransactionVOutIndex string `json:"bitcoinTransactionVOutIndex"`
	BitcoinTransactionVOutTo    string `json:"bitcoinTransactionVOutTo"`
	BitcoinTransactionVOutValue int64  `json:"bitcoinTransactionVOutValue"`
}

// Datastructure for Ethereum Transaction
type EthereumTransaction struct {
	EthereumTransactionHash             string    `json:"ethereumTransactionHash"`
	EthereumTransactionNonce            int64     `json:"ethereumTransactionNonce"`
	EthereumTransactionBlockHash        string    `json:"ethereumTransactionBlockHash"`
	EthereumTransactionBlockNumber      int64     `json:"ethereumTransactionBlockNumber"`
	EthereumTransactionTransactionIndex int64     `json:"ethereumTransactionTransactionIndex"`
	EthereumTransactionFrom             string    `json:"ethereumTransactionFrom"`
	EthereumTransactionTo               string    `json:"ethereumTransactionTo"`
	EthereumTransactionValue            BigNumber `json:"ethereumTransactionValue"`
	EthereumTransactionGasPrice         BigNumber `json:"ethereumTransactionGasPrice"`
	EthereumTransactionGas              BigNumber `json:"ethereumTransactionGas"`
	EthereumTransactionInput            string    `json:"ethereumTransactionInput"`
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
	EthereumTransactionReceiptBlockHash         string `json:"ethereumTransactionReceiptBlockHash"`
	EthereumTransactionReceiptBlockNumber       int64  `json:"ethereumTransactionReceiptBlockNumber"`
	EthereumTransactionReceiptTransactionHash   string `json:"ethereumTransactionReceiptTransactionHash"`
	EthereumTransactionReceiptTransactionIndex  int64  `json:"ethereumTransactionReceiptTransactionIndex"`
	EthereumTransactionReceiptFrom              string `json:"ethereumTransactionReceiptFrom"`
	EthereumTransactionReceiptTo                string `json:"ethereumTransactionReceiptTo"`
	EthereumTransactionReceiptCumulativeGasUsed int64  `json:"ethereumTransactionReceiptCumulativeGasUsed"`
	EthereumTransactionReceiptGasUsed           int64  `json:"ethereumTransactionReceiptGasUsed"`
	EthereumTransactionReceiptContractAddress   string `json:"ethereumTransactionReceiptContractAddress"`
	// Logs              []EthereumTransactionLog `json:"logs,omitempty"`
}

// Datastructure combining all the different types of transactions
type BlockTransaction struct {
	mixin.Model

	Address string `json:"address"`

	BitcoinTransaction
	BitcoinVIn
	BitcoinVOut
	EthereumTransaction
	EthereumTransactionReceipt

	Type          Type          `json:"type"`
	Status        ProcessStatus `json:"status"`
	Usage         Usage         `json:"usage"`
	Confirmations int64         `json:"confirmations"`
}
