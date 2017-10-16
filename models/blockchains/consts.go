package blockchains

// Type is the blockchain identifier
type Type string

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
	ReadingProcessStatus  ProcessStatus = "reading"
	FinishedProcessStatus ProcessStatus = "finished"
)

var BlockchainNamespace = "_blockchains"
