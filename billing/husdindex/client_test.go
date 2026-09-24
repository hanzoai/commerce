package husdindex

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	luxcrypto "github.com/luxfi/crypto"
)

// The Transfer topic constant MUST equal keccak256 of the event signature.
func TestTransferTopic0_Matches(t *testing.T) {
	got := "0x" + strings.ToLower(hexBytes(luxcrypto.Keccak256([]byte("Transfer(address,address,uint256)"))))
	if got != TransferTopic0 {
		t.Fatalf("TransferTopic0=%s, keccak=%s", TransferTopic0, got)
	}
}

func hexBytes(b []byte) string {
	const hexd = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, c := range b {
		out[i*2] = hexd[c>>4]
		out[i*2+1] = hexd[c&0x0f]
	}
	return string(out)
}

// rpcServer serves canned JSON-RPC results keyed by method, and records requests.
func rpcServer(t *testing.T, results map[string]any) (*httptest.Server, *[]map[string]any) {
	t.Helper()
	var reqs []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		reqs = append(reqs, req)
		method, _ := req["method"].(string)
		res, ok := results[method]
		if !ok {
			t.Errorf("unexpected rpc method %q", method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req["id"], "result": res})
	}))
	t.Cleanup(srv.Close)
	return srv, &reqs
}

func TestClient_BlockNumber(t *testing.T) {
	srv, _ := rpcServer(t, map[string]any{"eth_blockNumber": "0x64"}) // 100
	c := NewClient(srv.URL, "0xToken")
	n, err := c.BlockNumber(context.Background())
	if err != nil || n != 100 {
		t.Fatalf("BlockNumber=%d err=%v", n, err)
	}
}

func TestClient_BalanceOf(t *testing.T) {
	// 1e18 = 0xde0b6b3a7640000
	srv, reqs := rpcServer(t, map[string]any{"eth_call": "0x0000000000000000000000000000000000000000000000000de0b6b3a7640000"})
	c := NewClient(srv.URL, "0xToKeN")
	bal, err := c.BalanceOf(context.Background(), "0xAbCdef0000000000000000000000000000000001")
	if err != nil {
		t.Fatal(err)
	}
	if bal.String() != "1000000000000000000" {
		t.Fatalf("balance=%s", bal)
	}
	// The eth_call must target the token, with balanceOf(addr) calldata.
	params := (*reqs)[0]["params"].([]any)
	call := params[0].(map[string]any)
	if strings.ToLower(call["to"].(string)) != "0xtoken" {
		t.Errorf("call to=%v, want token", call["to"])
	}
	data := call["data"].(string)
	if !strings.HasPrefix(data, balanceOfSelector) || !strings.HasSuffix(strings.ToLower(data), "abcdef0000000000000000000000000000000001") {
		t.Errorf("balanceOf calldata wrong: %s", data)
	}
}

func TestClient_TransfersTo(t *testing.T) {
	watched := "0xe31e41e468893c44a4011d80b3315f1c362ba565"
	logs := []map[string]any{
		{
			"address": "0xtoken",
			"topics": []string{
				TransferTopic0,
				"0x000000000000000000000000" + "1111111111111111111111111111111111111111", // from
				"0x000000000000000000000000" + strings.TrimPrefix(watched, "0x"),          // to
			},
			"data":            "0x0000000000000000000000000000000000000000000000000de0b6b3a7640000", // 1e18
			"blockNumber":     "0x9",
			"transactionHash": "0xTX1",
			"logIndex":        "0x2",
		},
	}
	srv, reqs := rpcServer(t, map[string]any{"eth_getLogs": logs})
	c := NewClient(srv.URL, "0xtoken")
	trs, err := c.TransfersTo(context.Background(), []string{watched}, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(trs) != 1 {
		t.Fatalf("want 1 transfer, got %d", len(trs))
	}
	tr := trs[0]
	if tr.To != watched {
		t.Errorf("To=%s, want %s", tr.To, watched)
	}
	if tr.From != "0x1111111111111111111111111111111111111111" {
		t.Errorf("From=%s", tr.From)
	}
	if tr.ValueWei.String() != "1000000000000000000" {
		t.Errorf("Value=%s", tr.ValueWei)
	}
	if tr.Block != 9 || tr.LogIndex != 2 || tr.TxHash != "0xtx1" {
		t.Errorf("meta wrong: %+v", tr)
	}
	if tr.DedupKey() != "0xtx1:2" {
		t.Errorf("DedupKey=%s", tr.DedupKey())
	}
	// The filter must carry the padded recipient topic.
	filter := (*reqs)[0]["params"].([]any)[0].(map[string]any)
	topics := filter["topics"].([]any)
	if topics[0].(string) != TransferTopic0 {
		t.Errorf("topic0 wrong: %v", topics[0])
	}
}

func TestDecodeTransfer_BadTopics(t *testing.T) {
	_, err := decodeTransfer(rawLog{Topics: []string{TransferTopic0}, Data: "0x1"})
	if err == nil {
		t.Fatal("want error for wrong topic count")
	}
}

func TestPadAndTopic(t *testing.T) {
	if pad32("0x01") != strings.Repeat("0", 62)+"01" {
		t.Errorf("pad32=%s", pad32("0x01"))
	}
	if topicToAddr("0x000000000000000000000000abcdef0000000000000000000000000000000001") != "0xabcdef0000000000000000000000000000000001" {
		t.Error("topicToAddr wrong")
	}
}

// The ERC-20 metadata selectors MUST equal keccak256 of their signatures. A
// typo here would read some other function on the token contract and answer
// confident nonsense — which the deposit watcher then prices money with.
func TestERC20Selectors_MatchKeccak(t *testing.T) {
	for _, tc := range []struct{ sig, want string }{
		{"decimals()", decimalsSelector},
		{"symbol()", symbolSelector},
		{"balanceOf(address)", balanceOfSelector},
	} {
		got := "0x" + strings.ToLower(hexBytes(luxcrypto.Keccak256([]byte(tc.sig))[:4]))
		if got != tc.want {
			t.Errorf("selector for %s = %s, want %s", tc.sig, tc.want, got)
		}
	}
}

func TestClient_Decimals(t *testing.T) {
	srv, _ := rpcServer(t, map[string]any{
		"eth_call": "0x0000000000000000000000000000000000000000000000000000000000000006",
	})
	got, err := NewClient(srv.URL, "0xtoken").Decimals(context.Background())
	if err != nil {
		t.Fatalf("Decimals: %v", err)
	}
	if got != 6 {
		t.Fatalf("Decimals = %d, want 6", got)
	}
}

// A contract with no decimals() answers "0x". Reading that as 0 would value one
// base unit as a whole token, so it must be an error and not a number.
func TestClient_Decimals_EmptyReturnIsAnError(t *testing.T) {
	for _, empty := range []string{"0x", ""} {
		srv, _ := rpcServer(t, map[string]any{"eth_call": empty})
		if got, err := NewClient(srv.URL, "0xnotatoken").Decimals(context.Background()); err == nil {
			t.Fatalf("Decimals(%q) = (%d, nil), want an error", empty, got)
		}
	}
}

func TestClient_Symbol(t *testing.T) {
	// ABI dynamic string: offset=0x20, length=4, "USDC" right-padded.
	dynamic := "0x" +
		"0000000000000000000000000000000000000000000000000000000000000020" +
		"0000000000000000000000000000000000000000000000000000000000000004" +
		"5553444300000000000000000000000000000000000000000000000000000000"
	srv, _ := rpcServer(t, map[string]any{"eth_call": dynamic})
	got, err := NewClient(srv.URL, "0xtoken").Symbol(context.Background())
	if err != nil {
		t.Fatalf("Symbol: %v", err)
	}
	if got != "USDC" {
		t.Fatalf("Symbol = %q, want USDC", got)
	}
}

func TestDecodeABIString(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
		wantErr        bool
	}{
		{
			name: "dynamic string",
			in: "0x" +
				"0000000000000000000000000000000000000000000000000000000000000020" +
				"0000000000000000000000000000000000000000000000000000000000000004" +
				"5553445400000000000000000000000000000000000000000000000000000000",
			want: "USDT",
		},
		{
			// A handful of pre-standard tokens return a raw bytes32.
			name: "bytes32",
			in:   "0x4d4b520000000000000000000000000000000000000000000000000000000000",
			want: "MKR",
		},
		{name: "empty", in: "0x", wantErr: true},
		{name: "odd hex", in: "0x123", wantErr: true},
		{
			name: "offset past the end",
			in: "0x" +
				"00000000000000000000000000000000000000000000000000000000000000ff" +
				"0000000000000000000000000000000000000000000000000000000000000004",
			wantErr: true,
		},
		{
			name: "length past the end",
			in: "0x" +
				"0000000000000000000000000000000000000000000000000000000000000020" +
				"00000000000000000000000000000000000000000000000000000000000000ff",
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeABIString(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("decodeABIString(%q) = %q, want an error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeABIString: %v", err)
			}
			if got != tc.want {
				t.Fatalf("decodeABIString = %q, want %q", got, tc.want)
			}
		})
	}
}
