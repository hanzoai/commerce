package bitcoin

type Destination struct {
	Value   int64
	Address string
}

type Input struct {
	TxId        string
	OutputIndex int
}

type Sender struct {
	PrivateKey     string
	PublicKey      string
	Address        string
	TestNetAddress string
}
