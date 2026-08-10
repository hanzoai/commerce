package solana

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/custody"
	"github.com/hanzoai/commerce/billing/solanarpc"
)

// LIVE proofs against a real Solana cluster. Nothing here broadcasts.
//
//	SOL_LIVE=1 go test ./billing/custody/solana/ -run TestLive -v
//
// simulateTransaction is what makes these worth running: it EXECUTES the
// transaction against the cluster's real state and changes nothing. A fixture
// can only confirm we agree with ourselves about the bytes; the runtime is the
// only thing that can confirm a program will accept them.
const (
	liveRPC = "https://api.mainnet-beta.solana.com"
	// A long-lived, heavily funded mainnet account. It is used only as the
	// SOURCE of a simulated one-lamport transfer, which moves nothing: a
	// simulation is a read.
	liveFunded = "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM"
	liveDest   = "5tzFkiKscXHK5ZXCGbXZxdw7gTjjD1mBwuoFbhUvuAi9"
)

func liveCtx(t *testing.T) context.Context {
	t.Helper()
	if os.Getenv("SOL_LIVE") == "" {
		t.Skip("set SOL_LIVE=1 to reach a Solana cluster")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// simulate posts directly rather than going through Client.Simulate, because it
// needs sigVerify OFF — this proof is about whether the RUNTIME accepts our
// instruction, and the account it runs against is one whose key we do not have
// and must not have. Client.Simulate keeps verification on, which is the right
// default for a real sweep and the wrong one for this question.
func simulate(t *testing.T, ctx context.Context, raw []byte, sigVerify bool) (errField string, logs []string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "simulateTransaction",
		"params": []any{
			base64.StdEncoding.EncodeToString(raw),
			map[string]any{
				"commitment": "processed", "encoding": "base64",
				"sigVerify": sigVerify, "replaceRecentBlockhash": !sigVerify,
			},
		},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, liveRPC, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		Result struct {
			Value struct {
				Err  json.RawMessage `json:"err"`
				Logs []string        `json:"logs"`
			} `json:"value"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("simulate decode: %v", err)
	}
	if out.Error != nil {
		// An RPC-level error means the cluster could not even read the
		// transaction — a deserialization failure, which is the thing this test
		// most needs to catch.
		t.Fatalf("the cluster rejected our transaction before executing it: %s", out.Error.Message)
	}
	if len(out.Result.Value.Err) > 0 && string(out.Result.Value.Err) != "null" {
		return string(out.Result.Value.Err), out.Result.Value.Logs
	}
	return "", out.Result.Value.Logs
}

// unsigned wraps a drafted message with a placeholder signature, which is what
// a transaction looks like on the wire once Seal has attached a real one. Seal
// itself refuses a placeholder, correctly — that is why this test builds the
// envelope rather than calling it.
func unsigned(msg []byte) []byte {
	out := make([]byte, 0, 1+64+len(msg))
	out = append(out, 1)
	out = append(out, make([]byte, 64)...)
	return append(out, msg...)
}

// TestLiveRuntimeAcceptsOurTransfer is the positive proof: the Solana runtime
// runs our drafted transfer against real state and succeeds.
//
// Account ordering, header counts, compact-u16 lengths and the System Program's
// instruction encoding are all exercised here by the only authority that
// matters. Every one of them is invisible to a unit test that compares our
// bytes to our own expectations.
func TestLiveRuntimeAcceptsOurTransfer(t *testing.T) {
	ctx := liveCtx(t)
	c := New(liveRPC)

	held, err := c.rpc.Lamports(ctx, solanarpc.MustPublicKey(liveFunded))
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	t.Logf("%s holds %d lamports", liveFunded, held)
	if held == 0 {
		t.Skipf("%s is empty; that is news about the world, not about our code", liveFunded)
	}

	d, err := c.Draft(ctx, custody.Transfer{
		OrgID: "live", WalletID: "none", From: liveFunded, To: liveDest, Amount: big.NewInt(1),
	})
	if err != nil {
		t.Fatalf("Draft against live state: %v", err)
	}
	msg := d.Digests[0]
	t.Logf("drafted a %d-byte message", len(msg))

	errField, logs := simulate(t, ctx, unsigned(msg), false)
	if errField != "" {
		t.Fatalf("the runtime rejected our transfer: %s\nlogs: %v", errField, logs)
	}
	t.Logf("the Solana runtime executed our transfer against real state; logs: %v", logs)
}

// TestLiveSignatureVerifiesAtTheCluster runs the same path with verification ON
// and our own key.
//
// The account is unfunded, so execution must fail — but WHERE it fails is the
// point. A failure about funds means the cluster deserialized our transaction
// and verified our signature over our message before it ever looked at the
// balance. A failure about the signature or the encoding would mean the bytes
// are wrong.
func TestLiveSignatureVerifiesAtTheCluster(t *testing.T) {
	ctx := liveCtx(t)
	c := New(liveRPC)

	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 7)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	var pk solanarpc.PublicKey
	copy(pk[:], priv.Public().(ed25519.PublicKey))
	t.Logf("unfunded custody address: %s", pk)

	d, err := c.Draft(ctx, custody.Transfer{
		OrgID: "live", WalletID: "none", From: pk.String(), To: liveDest, Amount: big.NewInt(1),
	})
	if err != nil {
		// An empty account cannot pay a fee, which the draft is right to say.
		var noFee *custody.ErrNoFee
		if asNoFee(err, &noFee) {
			t.Logf("Draft correctly refused an unfunded address: %v", err)
			// Build the message anyway, so the cluster still gets to check the
			// bytes and the signature.
			hash, _, herr := c.rpc.Blockhash(ctx)
			if herr != nil {
				t.Fatal(herr)
			}
			bh := solanarpc.MustPublicKey(hash)
			dest := solanarpc.MustPublicKey(liveDest)
			m, _, berr := c.native(ctx, pk, dest, bh, custody.Transfer{Amount: big.NewInt(1)}, 0)
			if berr != nil {
				t.Fatal(berr)
			}
			d = c.draft(m, pk)
		} else {
			t.Fatal(err)
		}
	}

	raw, err := d.Seal([][]byte{ed25519.Sign(priv, d.Digests[0])})
	if err != nil {
		t.Fatalf("Seal refused our own valid signature: %v", err)
	}

	errField, logs := simulate(t, ctx, raw, true)
	if errField == "" {
		t.Fatal("an unfunded account somehow paid for a transfer; the simulation proves nothing")
	}
	t.Logf("cluster verified our signature and then failed on state, as it must: %s", errField)
	for _, bad := range []string{"Signature", "signature", "Deserialization", "deserialize", "Sanitize"} {
		if strings.Contains(errField, bad) {
			t.Fatalf("the cluster failed on our BYTES, not on the account's state: %s\nlogs: %v", errField, logs)
		}
	}
}
