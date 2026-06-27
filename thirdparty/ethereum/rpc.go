package ethereum

import (
	"fmt"
	"strings"

	"github.com/luxfi/geth/common/hexutil"

	"github.com/hanzoai/commerce/util/rand"
)

// NonceAt returns the transaction count (next nonce) for an address at the
// given block tag ("pending" is the correct tag for building the next tx).
// In Test mode the JSON-RPC layer returns "0x0", which decodes to nonce 0.
func (c Client) NonceAt(address, tag string) (uint64, error) {
	if tag == "" {
		tag = "pending"
	}
	id := rand.Int64()
	cmd := fmt.Sprintf(JsonRpcMessage, JsonRpcVersion, "eth_getTransactionCount", paramsToString(address, tag), id)
	jrr, err := c.Post(cmd, id)
	if err != nil {
		return 0, err
	}
	h := strings.Trim(string(jrr.Result), `"`)
	if h == "" || h == "0x" {
		return 0, nil
	}
	n, err := hexutil.DecodeUint64(h)
	if err != nil {
		return 0, fmt.Errorf("decode nonce %q: %w", h, err)
	}
	return n, nil
}

// SendRawTransaction submits a signed, RLP-encoded transaction (0x-hex) and
// returns its transaction hash.
func (c Client) SendRawTransaction(rawHex string) (string, error) {
	id := rand.Int64()
	cmd := fmt.Sprintf(JsonRpcMessage, JsonRpcVersion, "eth_sendRawTransaction", paramsToString(rawHex), id)
	jrr, err := c.Post(cmd, id)
	if err != nil {
		return "", err
	}
	return strings.Trim(string(jrr.Result), `"`), nil
}
