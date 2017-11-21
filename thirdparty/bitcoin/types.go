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

type GetRawTransactionResponseResult struct {
	Txid      string
	Hash      string
	Version   int
	Size      int
	Vsize     int
	Locktime  int
	Vin       []GetRawTransactionResponseResultInput
	Vout      []GetRawTransactionResponseResultOutput
	Hex       string
	Blockhash string
	Time      int64
	Blocktime int64
}

type GetRawTransactionResponseResultInput struct {
	Txid        string
	Vout        int
	Scriptsig   GetRawTransactionResponseResultInputSig
	Txinwitness []string
	Sequence    int64
}

type GetRawTransactionResponseResultInputSig struct {
	Asm string
	Hex string
}

type GetRawTransactionResponseResultOutput struct {
	N            int
	Value        float64
	Scriptpubkey GetRawTransactionResponseResultOutputScriptPubKey
}

type GetRawTransactionResponseResultOutputScriptPubKey struct {
	Asm        string
	Hex        string
	ReqSigs    int
	ScriptType string `json:"type"`
	Addresses  []string
}

type GetRawTransactionResponseError struct {
	Code    int
	Message string
}
