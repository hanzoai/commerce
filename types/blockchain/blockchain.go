package blockchain

import "hanzo.io/config"

// Type is the blockchain identifier
type Type string

const (
	Bitcoin  Type = "bitcoin"
	EOS      Type = "eos"
	Ethereum Type = "ethereum"
	Stellar  Type = "stellar"
)

type Network struct {
	Blockchain Type   `json:"type"`
	Name       string `json:"name"`
	ID         string `json:"id"`
	URL        string `json:"url"`
	Port       int    `json:"port"`
}

type Token struct {
	Name     string `json:"name"`
	Decimals int    `json:"decimals"`
	Symbol   int    `json:"symbol"`
}

var (
	BitcoinMainnet = Network{
		Blockchain: Bitcoin,
		Name:       "bitcoin-mainnet",
		ID:         "",
		URL:        "",
		Port:       -1,
	}

	BitcoinTestnet = Network{
		Blockchain: Bitcoin,
		Name:       "bitcoin-testnet",
		ID:         "",
		URL:        "",
		Port:       -1,
	}

	EthereumMainnet = Network{
		Blockchain: Ethereum,
		Name:       "ethereum-mainnet",
		ID:         "1",
		URL:        "https://mainnet.infura.io/" + config.Infura.APIKey,
		Port:       443,
	}

	Morden = Network{
		Blockchain: Ethereum,
		Name:       "ethereum-morden",
		ID: "2	",
		URL:  "",
		Port: -1,
	}

	Ropsten = Network{
		Blockchain: Ethereum,
		Name:       "ethereum-ropsten",
		ID:         "3",
		URL:        "https://ropsten.infura.io/" + config.Infura.APIKey,
		Port:       443,
	}

	Rinkeby = Network{
		Blockchain: Ethereum,
		Name:       "ethereum-ropsten",
		ID:         "4",
		URL:        "",
		Port:       -1,
	}

	Kovan = Network{
		Blockchain: Ethereum,
		Name:       "ethereum-kovan",
		ID:         "42",
		URL:        "",
		Port:       -1,
	}

	EOSMainnet = Network{
		Blockchain: EOS,
		Name:       "eos-mainnet",
		ID:         "42",
		URL:        "https://api.eosnewyork.io",
		Port:       80,
	}

	Jungle = Network{
		Blockchain: EOS,
		Name:       "eos-jungle",
		ID:         "42",
		URL:        "http://jungle2.cryptolions.io",
		Port:       80,
	}
)
