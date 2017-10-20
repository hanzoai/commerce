package ethereum

import (
	ej "encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"appengine"
	"appengine/urlfetch"

	"hanzo.io/util/json"
	"hanzo.io/util/log"
)

type Client struct {
	ctx        appengine.Context
	httpClient *http.Client

	address string
}

type JsonRpcResponse struct {
	Id             int64         `json:"id"`
	JsonRpcVersion string        `json:"jsonrpc"`
	Result         ej.RawMessage `json:"result"`
}

var JsonRpcVersion = "2.0"
var JsonRpcMessage = `{"jsonrpc":"%s","method":"%s","params":["%s"],"id":%v}`

func New(ctx appengine.Context, address string) Client {
	httpClient := urlfetch.Client(ctx)
	httpClient.Transport = &urlfetch.Transport{
		Context:                       ctx,
		Deadline:                      time.Duration(55) * time.Second,
		AllowInvalidServerCertificate: appengine.IsDevAppServer(),
	}

	return Client{ctx, httpClient, address}
}

func (c Client) SendRawTransaction(signedTx string) (*JsonRpcResponse, error) {
	log.Info("Sending signed transaction: %s", signedTx, c.ctx)
	jsonRpcCommand := fmt.Sprintf(JsonRpcMessage, JsonRpcVersion, "eth_sendRawTransaction", signedTx)

	log.Info("Posting command to Geth Node '%s': %s", c.address, jsonRpcCommand, c.ctx)
	res, err := c.httpClient.Post(c.address, "application/json", strings.NewReader(jsonRpcCommand))
	if err != nil {
		return nil, err
	}

	jrr := &JsonRpcResponse{}
	if err := json.Decode(res.Body, jrr); err != nil {
		return nil, err
	}

	log.Info("Received response from Geth Node '%s': %v", c.address, jrr, c.ctx)

	return jrr, nil
}
