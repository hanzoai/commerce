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
	Hash             string  `json:"hash"`
	Nonce            int64   `json:"nonce"`
	BlockHash        string  `json:"blockHash"`
	BlockNumber      int64   `json:"blockNumber"`
	TransactionIndex int64   `json:"transactionIndex"`
	From             string  `json:"from"`
	To               string  `json:"to"`
	Value            float64 `json:"value"`
	GasPrice         float64 `json:"gasPrice"`
	Gas              float64 `json:"gas"`
	Input            string  `json:"input"`
}

// Datastructure for Bitcoin Transaction Receipt
type BitcoinTransactionReceipt struct {
}

// Datastructure for Ethereum Transaction Logs
type EthereumTransactionLog struct {
	LogIndex         int64  `json:"logIndex"`
	BlockHash        string `json:"blockHash"`
	BlockNumber      int64  `json:"blockNumber"`
	TransactionHash  string `json:"transactionHash"`
	TransactionIndex int64  `json:"transactionIndex"`
	Address          string `json:"address"`
	Data             string `json:"data"`
	Topics           string `json:"topics"`
}

// Datastructure for Ethereum Transaction Receipt
type EthereumTransactionReceipt struct {
	BlockHash         string                   `json:"blockHash"`
	BlockNumber       int64                    `json:"blockNumber"`
	TransactionHash   string                   `json:"transactionHash"`
	TransactionIndex  int64                    `json:"transactionIndex"`
	From              string                   `json:"from"`
	To                string                   `json:"to"`
	CumulativeGasUsed float64                  `json:"cumulativeGasUsed"`
	GasUsed           float64                  `json:"gasUsed"`
	ContractAddress   string                   `json:"contractAddress"`
	Logs              []EthereumTransactionLog `json:"logs,omitempty"`
}

// Datastructure combining all the different types of transactions
type BlockTransaction struct {
	mixin.Model

	BitcoinTransaction  BitcoinTransaction  `json:"bitcoinTransaction,omitempty"`
	EthereumTransaction EthereumTransaction `json:"ethereumTransaction,omitempty"`

	EthereumTransactionReceipt EthereumTransactionReceipt `json:"ethereumTransactionReceipt,omitempty"`

	Type Type `json:"type"`
}
