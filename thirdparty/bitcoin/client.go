package bitcoin

import (
	"appengine"
	"appengine/urlfetch"
	"bytes"
	"encoding/hex"
	ej "encoding/json"

	"errors"
	"fmt"
	"hanzo.io/util/json"
	"hanzo.io/util/log"
	"hanzo.io/util/rand"
	"net/http"
	"time"
)

var JsonRpcVersion = "1.0"
var JsonRpcMessage = `{"jsonrpc":"%s","id":%v,"method":"%s","params":%s}`

type BitcoinClient struct {
	ctx        appengine.Context
	httpClient *http.Client
	host       string
	IsTest     bool
	Commands   []string
	Username   string
	Password   string
}

var IsTest = false

// Flip to Universal Test Mode
func Test(b bool) bool {
	IsTest = b
	return b
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

var IdMismatch = errors.New("Ids do not match!")

// New creates a new RPC client based on the provided connection configuration
// details.  The notification handlers parameter may be nil if you are not
// interested in receiving notifications and will be ignored if the
// configuration is set to run in HTTP POST mode.
func NewRpcClient(ctx appengine.Context, host, username, password string, testMode bool) (BitcoinClient, error) {
	httpClient := urlfetch.Client(ctx)
	httpClient.Transport = &urlfetch.Transport{
		Context:                       ctx,
		Deadline:                      time.Duration(55) * time.Second,
		AllowInvalidServerCertificate: appengine.IsDevAppServer(),
	}

	return BitcoinClient{ctx, httpClient, host, testMode, []string{}, username, password}, nil
}

func paramsToString(parts ...interface{}) string {
	str, err := ej.Marshal(parts)
	if err != nil {
		panic(err)
	}

	return string(str)
}

func (btcc *BitcoinClient) SendRawTransaction(rawTransaction []byte) (*JsonRpcResponse, error) {
	id := rand.Int64()
	jsonRpcCommand := fmt.Sprintf(JsonRpcMessage, JsonRpcVersion, id, "sendrawtransaction", paramsToString(hex.EncodeToString(rawTransaction)))

	res, err := btcc.Post(jsonRpcCommand, id)

	return res, err
}

func (btcc *BitcoinClient) GetRawTransaction(txId string) (*JsonRpcResponse, error) {
	id := rand.Int64()
	jsonRpcCommand := fmt.Sprintf(JsonRpcMessage, JsonRpcVersion, id, "getrawtransaction", paramsToString(txId, true))

	res, err := btcc.Post(jsonRpcCommand, id)

	return res, err
}

// Flip to Test Mode
func (c BitcoinClient) Test(b bool) bool {
	c.IsTest = b
	return b
}

func (c BitcoinClient) Post(jsonRpcCommand string, id int64) (*JsonRpcResponse, error) {
	c.Commands = append(c.Commands, jsonRpcCommand)

	// I dunno if this is appropriate for the bitcoin junk but it sure isn't
	// right now
	/*if c.IsTest || IsTest {
		jrr := &JsonRpcResponse{Result: ej.RawMessage([]byte(`"0x0"`))}
		return jrr, nil
	}*/

	bodyReader := bytes.NewReader([]byte(jsonRpcCommand))
	httpReq, err := http.NewRequest("POST", c.host, bodyReader)
	if err != nil {
		return nil, err
	}
	httpReq.Close = true
	httpReq.Header.Set("Content-Type", "application/json")

	// Configure basic access authorization.
	httpReq.SetBasicAuth(c.Username, c.Password)
	log.Info("Posting command to Bitcoin Node '%s': '%s'", c.host, jsonRpcCommand, c.ctx)
	res, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	jrr := &JsonRpcResponse{}

	if err := json.Decode(res.Body, jrr); err != nil {
		return nil, err
	}

	log.Info("Received response from bitcoin Node '%s': %v", c.host, json.Encode(jrr), c.ctx)

	// This could mean there's a man in the middle attack?
	if jrr.Id != id {
		return nil, IdMismatch
	}

	if jrr.Error.Code != 0 {
		return jrr, errors.New(jrr.Error.Message)
	}
	return jrr, nil
}
