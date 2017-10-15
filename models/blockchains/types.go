package blockchains

// Address is the blockchain address + type
type Address struct {
	// Address on the blockchain
	Address string `json:"address"`

	// Which blockchain contains the address
	Type Type `json:"type"`
}
