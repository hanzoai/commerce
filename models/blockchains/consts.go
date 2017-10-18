package blockchains

// Type is the blockchain identifier
type Type string
type BigNumber string

const (
	// Ethereum Blockchain
	EthereumType Type = "ethereum"

	// Ethereum Default Testnet
	EthereumRopsten Type = "ethereum-ropsten"

	// Bitcoin Blockchain
	BitcoinType Type = "bitcoin"
)

type ProcessStatus string

const (
	ReadingProcessStatus   ProcessStatus = "reading"
	PendingProcessStatus   ProcessStatus = "pending"
	ConfirmedProcessStatus ProcessStatus = "confirmed"
)

type Usage string

const (
	SenderUsage   Usage = "sender"
	ReceiverUsage Usage = "receiver"
)

var BlockchainNamespace = "_blockchains"
