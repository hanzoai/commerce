package smartcontract

import (
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/currency"
	"hanzo.io/types/blockchain"
)

// type Type string

// const (
// 	Generic Type = "generic"
// 	Game    Type = "game"
// 	Storage Type = "storage"
// 	Token   Type = "token"
// 	Wallet  Type = "wallet"
// )

type SmartContract struct {
	mixin.Model

	// Type            Type               `json:"type"`
	Name            string             `json:"name"`
	Blockchain      blockchain.Type    `json:"blockchain"`
	Network         blockchain.Network `json:"network"`
	PrivateKey      string             `json:"privateKey"`
	ABI             string             `json:"abi"`
	Methods         []string           `json:"methods"`
	Account         string             `json:"account"`
	Address         string             `json:"address"`
	TransactionHash string             `json:"transactionHash"`

	Balance  string        `json:"balance"`
	Currency currency.Type `json:"currency,omitempty"`

	Token blockchain.Token `json:"token,omitempty"`
}
