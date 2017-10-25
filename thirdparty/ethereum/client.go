package ethereum

import (
	ej "encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"appengine"
	"appengine/urlfetch"

	"hanzo.io/datastore"
	"hanzo.io/models/blockchains/blocktransaction"
	"hanzo.io/thirdparty/ethereum/go-ethereum/common"
	"hanzo.io/thirdparty/ethereum/go-ethereum/common/hexutil"
	"hanzo.io/thirdparty/ethereum/go-ethereum/core/types"
	"hanzo.io/thirdparty/ethereum/go-ethereum/crypto"
	"hanzo.io/thirdparty/ethereum/go-ethereum/rlp"
	"hanzo.io/util/json"
	"hanzo.io/util/log"
	"hanzo.io/util/rand"

	. "hanzo.io/models/blockchains"
)

type Client struct {
	ctx        appengine.Context
	httpClient *http.Client

	address string

	IsTest   bool
	Commands []string
}

type JsonRpcError struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

type JsonRpcResponse struct {
	Id             int64         `json:"id"`
	JsonRpcVersion string        `json:"jsonrpc"`
	Result         ej.RawMessage `json:"result"`
	Error          JsonRpcError  `json:"error"`
}

var JsonRpcVersion = "2.0"
var JsonRpcMessage = `{"jsonrpc":"%s","method":"%s","params":[%s],"id":%v}`

var IdMismatch = errors.New("Ids do not match!")
var InvalidChainId = errors.New("Invalid ChainId")

var IsTest = false

// Flip to Universal Test Mode
func Test(b bool) bool {
	IsTest = b
	return b
}

// Create a new Ethereum JSON-RPC client
func New(ctx appengine.Context, address string) Client {
	httpClient := urlfetch.Client(ctx)
	httpClient.Transport = &urlfetch.Transport{
		Context:                       ctx,
		Deadline:                      time.Duration(55) * time.Second,
		AllowInvalidServerCertificate: appengine.IsDevAppServer(),
	}

	return Client{ctx, httpClient, address, false, []string{}}
}

func paramsToString(parts ...string) string {
	return `"` + strings.Join(parts, `","`) + `"`
}

// Flip to Test Mode
func (c Client) Test(b bool) bool {
	c.IsTest = b
	return b
}

// Post a JSON-RPC Command
func (c Client) Post(jsonRpcCommand string, id int64) (*JsonRpcResponse, error) {
	c.Commands = append(c.Commands, jsonRpcCommand)

	if c.IsTest || IsTest {
		jrr := &JsonRpcResponse{Result: ej.RawMessage([]byte(`"0x0"`))}
		return jrr, nil
	}

	log.Info("Posting command to Geth Node '%s': '%s'", c.address, jsonRpcCommand, c.ctx)
	res, err := c.httpClient.Post(c.address, "application/json", strings.NewReader(jsonRpcCommand))
	if err != nil {
		return nil, err
	}

	jrr := &JsonRpcResponse{}

	if err := json.Decode(res.Body, jrr); err != nil {
		return nil, err
	}

	log.Info("Received response from Geth Node '%s': %v", c.address, json.Encode(jrr), c.ctx)

	// This could mean there's a man in the middle attack?
	if jrr.Id != id {
		return nil, IdMismatch
	}

	if jrr.Error.Code != 0 {
		return jrr, errors.New(jrr.Error.Message)
	}
	return jrr, nil
}

// Send a transaction
func (c Client) SendTransaction(chainId ChainId, pk, from string, to string, amount, gasLimit, gasPrice *big.Int, data []byte) (string, error) {
	// Figure out what chain we are using
	var chainType Type
	switch chainId {
	case MainNet:
		chainType = EthereumType
	case Morden:
		chainType = EthereumMordenType
	case Ropsten:
		chainType = EthereumRopstenType
	default:
		return "", InvalidChainId
	}

	ctx := c.ctx
	// Setup defaults
	if gasLimit.Cmp(big.NewInt(0)) <= 0 {
		gasLimit = big.NewInt(defaultGas)
	}

	if gasPrice.Cmp(big.NewInt(0)) <= 0 {
		if price, err := c.GasPrice(); err != nil || price.Cmp(big.NewInt(0)) == 0 {
			log.Error("Could Not Determine Gas Price '%s': %v", price, err, ctx)
			gasPrice = big.NewInt(defaultGasPrice)
		} else {
			log.Error("Current Gas Price is '%s'", price, ctx)
			gasPrice = price
		}
	}

	// Decode the private key
	privKey, err := crypto.HexToECDSA(pk)
	if err != nil {
		log.Error("Could Not Decode Hex '%s': %v", pk, err, ctx)
		return "", err
	}

	// Create a queued transaction to record this happened
	db := datastore.New(ctx)

	// Get Nonce from db
	nonce := uint64(0)

	nonceBt := blocktransaction.New(db)
	ok, err := nonceBt.Query().Filter("Type=", chainType).Filter("EthereumTransactionFrom=", from).Order("-EthereumTransactionNonce").Get()
	if err != nil {
		log.Error("Could Not Find Last BlockTransaction Due to Error: %v", err, ctx)
		return "", err
	} else if ok {
		nonce = uint64(nonceBt.EthereumTransactionNonce) + 1
	}

	// Create a signer for the particular chain using the modern signature
	// algorithm
	signer := types.NewEIP155Signer(big.NewInt(int64(chainId)))
	tx := types.NewTransaction(nonce, common.HexToAddress(to), amount, gasLimit, gasPrice, data)

	log.Info("Unsigned Transaction: %v", tx.String(), ctx)

	// Sign the transaction
	signedTx, err := types.SignTx(tx, signer, privKey)
	if err != nil {
		log.Error("Could Not Sign Transaction: %v", err, ctx)
		return "", err
	}

	log.Info("Signed Transaction: %v", signedTx.String(), ctx)

	// get RLP of transaction
	bytes, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		log.Error("Could Not Encode To Bytes: %v", err, ctx)
		return "", err
	}

	txHex := common.ToHex(bytes)

	id := rand.Int64()

	log.Info("Sending signed transaction: %s", txHex, c.ctx)
	jsonRpcCommand := fmt.Sprintf(JsonRpcMessage, JsonRpcVersion, "eth_sendRawTransaction", paramsToString(txHex), id)

	jrr, err := c.Post(jsonRpcCommand, id)
	if err != nil {
		return "", err
	}

	hash := string(jrr.Result)
	hash = hash[1 : len(hash)-1]

	bt := blocktransaction.New(db)
	bt.Id_ = string(chainType) + "/" + from + "/" + hash
	bt.UseStringKey = true
	bt.EthereumTransactionHash = hash
	bt.EthereumTransactionNonce = int64(nonce)
	bt.EthereumTransactionFrom = from
	bt.EthereumTransactionTo = to
	bt.EthereumTransactionValue = BigNumber(amount.String())
	bt.EthereumTransactionGasPrice = BigNumber(amount.String())
	bt.EthereumTransactionGas = BigNumber(gasLimit.String())

	bt.Address = from
	bt.Usage = SenderUsage
	bt.Type = chainType
	bt.Status = QueuedProcessStatus

	if err := bt.Create(); err != nil {
		return "", err
	}

	return hash, nil
}

// Get the current average gasprice
func (c Client) GasPrice() (*big.Int, error) {
	id := rand.Int64()

	log.Info("Getting Gas Price", c.ctx)
	jsonRpcCommand := fmt.Sprintf(JsonRpcMessage, JsonRpcVersion, "eth_gasPrice", "", id)

	jrr, err := c.Post(jsonRpcCommand, id)
	if err != nil {
		return big.NewInt(0), err
	}

	priceHex := string(jrr.Result)
	priceHex = priceHex[1 : len(priceHex)-1]

	log.Info("Gas Price is %s", priceHex, c.ctx)

	a, err := hexutil.DecodeBig(priceHex)
	if err != nil {
		return nil, err
	}

	// 1 is the min price
	if a.Cmp(big.NewInt(0)) == 0 {
		return big.NewInt(1), nil
	}

	return a, nil
}
