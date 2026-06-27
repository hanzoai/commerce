package ethereum

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/luxfi/geth/common"

	blockchainutil "github.com/hanzoai/commerce/util/blockchain"
)

func TestEncodeERC20Transfer(t *testing.T) {
	to := common.HexToAddress("0x00000000000000000000000000000000000000ff")
	data := EncodeERC20Transfer(to, big.NewInt(1))

	if len(data) != 4+32+32 {
		t.Fatalf("calldata len=%d, want 68", len(data))
	}
	// transfer(address,uint256) selector.
	if got := fmt.Sprintf("%x", data[:4]); got != "a9059cbb" {
		t.Errorf("selector=%s, want a9059cbb", got)
	}
	// recipient is right-aligned in the 32-byte arg slot.
	if data[4+31] != 0xff {
		t.Errorf("recipient padding wrong: %x", data[4:4+32])
	}
	// amount is right-aligned in the next slot.
	if data[4+32+31] != 0x01 {
		t.Errorf("amount padding wrong: %x", data[4+32:])
	}
}

// TestTransferToken_RealTxHashShape drives the full on-chain path against a
// stub JSON-RPC server: it signs a real ERC-20 transfer with a generated
// treasury key, submits it, and returns the chain's transaction hash.
func TestTransferToken_RealTxHashShape(t *testing.T) {
	const cannedHash = "0x1111111111111111111111111111111111111111111111111111111111111111"
	var rawTxSeen string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Method string        `json:"method"`
			Params []interface{} `json:"params"`
			ID     int64         `json:"id"`
		}
		_ = json.Unmarshal(body, &req)

		var result string
		switch req.Method {
		case "eth_getTransactionCount":
			result = "0x1"
		case "eth_gasPrice":
			result = "0x3b9aca00" // 1 gwei
		case "eth_sendRawTransaction":
			if len(req.Params) > 0 {
				rawTxSeen, _ = req.Params[0].(string)
			}
			result = cannedHash
		default:
			result = "0x0"
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":%q}`, req.ID, result)
	}))
	defer srv.Close()

	privHex, _, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	got, err := transferToken(context.Background(), blockchainutil.TokenTransfer{
		ChainID:      36900, // Hanzo EVM
		RPCURL:       srv.URL,
		TokenAddress: "0x000000000000000000000000000000000000dead",
		TreasuryKey:  privHex,
		To:           "0x00000000000000000000000000000000000000ff",
		AmountWei:    big.NewInt(1_000_000),
	})
	if err != nil {
		t.Fatalf("transferToken: %v", err)
	}
	if got != cannedHash {
		t.Errorf("tx hash=%s, want %s", got, cannedHash)
	}
	if !regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`).MatchString(got) {
		t.Errorf("tx hash %q is not a 32-byte hash shape", got)
	}
	// The submitted raw transaction must embed the ERC-20 transfer calldata.
	if !strings.Contains(strings.ToLower(rawTxSeen), "a9059cbb") {
		t.Errorf("submitted raw tx missing ERC-20 transfer selector: %s", rawTxSeen)
	}
}

func TestTransferToken_RejectsZeroAmount(t *testing.T) {
	privHex, _, _, _ := GenerateKeyPair()
	if _, err := transferToken(context.Background(), blockchainutil.TokenTransfer{
		ChainID:      36900,
		RPCURL:       "http://127.0.0.1:0",
		TokenAddress: "0xdead",
		TreasuryKey:  privHex,
		To:           "0xff",
		AmountWei:    big.NewInt(0),
	}); err == nil {
		t.Fatal("expected error for zero amount")
	}
}

func TestTransferToken_RejectsBadKey(t *testing.T) {
	if _, err := transferToken(context.Background(), blockchainutil.TokenTransfer{
		ChainID:      36900,
		RPCURL:       "http://127.0.0.1:0",
		TokenAddress: "0xdead",
		TreasuryKey:  "not-a-hex-key",
		To:           "0xff",
		AmountWei:    big.NewInt(1),
	}); err == nil {
		t.Fatal("expected error for invalid treasury key")
	}
}
