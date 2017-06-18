package obj

type Address struct {
	Id            string // Resource ID
	Address       string // Bitcoin, Litecoin, or Ethereum address
	Name          string // User defined label for this address
	Network       string // Name of blockchain
	Created_At    string
	Updated_At    string
	Resource      string
	Resource_Path string
}
