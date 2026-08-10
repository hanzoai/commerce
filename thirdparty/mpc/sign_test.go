package mpc

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/billing/custody"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// fakeMPCD stands in for mpcd. It records the body it was sent and answers with
// whatever the test told it to.
type fakeMPCD struct {
	body   map[string]any
	auth   string
	path   string
	status int    // HTTP status
	reply  string // raw JSON body
}

func (s *fakeMPCD) serve(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.path = r.URL.Path
		s.auth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		s.body = map[string]any{}
		if err := json.Unmarshal(raw, &s.body); err != nil {
			t.Errorf("mpcd got a body that is not JSON: %v", err)
		}
		code := s.status
		if code == 0 {
			code = 200
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		io.WriteString(w, s.reply)
	}))
}

func processorFor(url string) *MPCProcessor {
	return NewProcessor(Config{KMSEndpoint: "https://kms.example", MPCEndpoint: url, APIKey: "k"})
}

func hexsig(n int) string { return strings.Repeat("ab", n) }

// TestSignSendsTheNetworkAndNotACurve is the whole reason this client was
// rewritten rather than repointed.
//
// mpcd derives the curve from `network` deliberately, "so that every caller
// cannot get it independently wrong". A client that sends `key_type` instead —
// which is what luxfi/mpc's own stale integration test and lux/wallet's custody
// adapter both still do — is refused with 400 "unknown network: (empty)",
// because the daemon decodes the unknown field into nothing.
func TestSignSendsTheNetworkAndNotACurve(t *testing.T) {
	s := &fakeMPCD{reply: `{"session_id":"sign-1","signature":"` + hexsig(65) + `","status":"signed"}`}
	srv := s.serve(t)
	defer srv.Close()

	digest := make([]byte, 32)
	for i := range digest {
		digest[i] = byte(i)
	}
	out, err := processorFor(srv.URL).Sign(context.Background(), "acme", "w1", custody.Ethereum, digest)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 65 {
		t.Fatalf("secp256k1 signature is %d bytes, want 65", len(out))
	}

	if s.path != "/sign" {
		t.Errorf("posted to %q, want /sign", s.path)
	}
	if s.auth != "Bearer k" {
		t.Errorf("Authorization %q, want the bearer token", s.auth)
	}
	if got := s.body["network"]; got != "ethereum" {
		t.Errorf("network=%v, want ethereum", got)
	}
	if got := s.body["payload_hash"]; got != hex.EncodeToString(digest) {
		t.Errorf("payload_hash=%v, want the digest as bare hex", got)
	}
	if got := s.body["org_id"]; got != "acme" {
		t.Errorf("org_id=%v", got)
	}
	if got := s.body["wallet_id"]; got != "w1" {
		t.Errorf("wallet_id=%v", got)
	}
	if s.body["idempotency_key"] == "" || s.body["idempotency_key"] == nil {
		t.Error("idempotency_key is required by mpcd and was not sent")
	}
	// The fields whose PRESENCE is the bug.
	for _, dead := range []string{"key_type", "curve", "algo", "scheme", "chain_id"} {
		if _, ok := s.body[dead]; ok {
			t.Errorf("sent %q, which mpcd does not route on", dead)
		}
	}
}

// TestSignRefusesRejectedAtHTTP200 is the failure an ordinary HTTP client would
// read as success.
//
// mpcd reserves 503 for having no quorum, so a signing FAILURE comes back as
// 200 with status "rejected" and an empty signature. Trusting the status code
// would hand an empty byte slice to a chain encoder.
func TestSignRefusesRejectedAtHTTP200(t *testing.T) {
	s := &fakeMPCD{reply: `{"session_id":"sign-9","signature":"","status":"rejected","error":"signing timeout"}`}
	srv := s.serve(t)
	defer srv.Close()

	_, err := processorFor(srv.URL).Sign(context.Background(), "acme", "w1", custody.Ethereum, make([]byte, 32))
	if err == nil {
		t.Fatal("accepted a rejected signing session as success")
	}
	for _, want := range []string{"rejected", "sign-9", "signing timeout"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should carry %q, got: %v", want, err)
		}
	}
}

// TestSignChecksSignatureLength catches a signer that answered on the wrong
// curve. It is the only property checkable without the transaction, and it
// stops a 64-byte Ed25519 signature being read as a truncated recoverable one.
func TestSignChecksSignatureLength(t *testing.T) {
	for _, tc := range []struct {
		name    string
		network custody.Network
		bytes   int
		ok      bool
	}{
		{"evm gets 65", custody.Ethereum, 65, true},
		{"evm gets 64", custody.Ethereum, 64, false},
		{"evm gets 32", custody.Ethereum, 32, false},
		{"solana gets 64", custody.Solana, 64, true},
		{"solana gets 65", custody.Solana, 65, false},
		{"bitcoin gets 65", custody.Bitcoin, 65, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeMPCD{reply: `{"session_id":"s","signature":"` + hexsig(tc.bytes) + `","status":"signed"}`}
			srv := s.serve(t)
			defer srv.Close()
			// Ed25519 networks sign a message, not a fixed-width digest, so give
			// each network something it will accept.
			payload := make([]byte, 32)
			_, err := processorFor(srv.URL).Sign(context.Background(), "acme", "w1", tc.network, payload)
			if tc.ok && err != nil {
				t.Fatalf("refused a valid %d-byte signature: %v", tc.bytes, err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("accepted a %d-byte signature on %s", tc.bytes, tc.network)
			}
		})
	}
}

// TestSignRefusesBadDigestLength fails in our own words rather than returning
// someone else's 400 from several layers up.
func TestSignRefusesBadDigestLength(t *testing.T) {
	s := &fakeMPCD{reply: `{"status":"signed","signature":"` + hexsig(65) + `"}`}
	srv := s.serve(t)
	defer srv.Close()
	p := processorFor(srv.URL)

	for _, n := range []int{0, 31, 33, 64} {
		if _, err := p.Sign(context.Background(), "acme", "w1", custody.Ethereum, make([]byte, n)); err == nil {
			t.Errorf("accepted a %d-byte digest on a secp256k1 network", n)
		}
	}
	// Ed25519 signs the message itself, so any non-empty length is legitimate.
	if _, err := p.Sign(context.Background(), "acme", "w1", custody.Solana, make([]byte, 215)); err != nil {
		s2 := &fakeMPCD{reply: `{"status":"signed","signature":"` + hexsig(64) + `"}`}
		srv2 := s2.serve(t)
		defer srv2.Close()
		if _, err := processorFor(srv2.URL).Sign(context.Background(), "acme", "w1", custody.Solana, make([]byte, 215)); err != nil {
			t.Errorf("refused a full Solana message: %v", err)
		}
	}
}

func TestSignRefusesUnknownNetworkAndMissingIdentity(t *testing.T) {
	s := &fakeMPCD{reply: `{"status":"signed","signature":"` + hexsig(65) + `"}`}
	srv := s.serve(t)
	defer srv.Close()
	p := processorFor(srv.URL)
	d := make([]byte, 32)

	if _, err := p.Sign(context.Background(), "acme", "w1", custody.Network("avalanche"), d); err == nil {
		t.Error("accepted avalanche, which mpcd's alias table does not know")
	}
	if _, err := p.Sign(context.Background(), "", "w1", custody.Ethereum, d); err == nil {
		t.Error("accepted a sign with no org_id, which mpcd requires")
	}
	if _, err := p.Sign(context.Background(), "acme", "", custody.Ethereum, d); err == nil {
		t.Error("accepted a sign with no wallet_id")
	}
}

// TestIdempotencyKeyIsContentAddressed is a money-safety property, not a
// convenience.
//
// mpcd returns the STORED signature for a repeated key and refuses a repeated
// key with different content (409). Deriving the key from the content gets both
// halves: a resumed sweep collects the signature it already paid for, and two
// different transfers can never collide. A random key would leave a signature
// after a timeout that the caller can never ask for again, and the retry would
// sign a second, different transaction over the same coins.
func TestIdempotencyKeyIsContentAddressed(t *testing.T) {
	d1, d2 := []byte("digest-one......................."), []byte("digest-two.......................")

	base := idemKey("acme", "w1", custody.Ethereum, d1)
	if base != idemKey("acme", "w1", custody.Ethereum, d1) {
		t.Fatal("the same request produced two keys; a retry would sign twice")
	}
	for name, other := range map[string]string{
		"different digest":  idemKey("acme", "w1", custody.Ethereum, d2),
		"different org":     idemKey("other", "w1", custody.Ethereum, d1),
		"different wallet":  idemKey("acme", "w2", custody.Ethereum, d1),
		"different network": idemKey("acme", "w1", custody.Base, d1),
	} {
		if other == base {
			t.Errorf("%s collided onto the same idempotency key", name)
		}
	}
	// Length-prefixing: these two tuples concatenate to the same bytes without it.
	if idemKey("ab", "c", custody.Ethereum, d1) == idemKey("a", "bc", custody.Ethereum, d1) {
		t.Error("two different (org, wallet) pairs flattened to one key")
	}
}

// TestSpendingMethodsRefuse pins the removal of the dead
// POST /api/v1/transactions calls.
//
// The methods stay because CryptoProcessor requires them. What must never come
// back is a call to a route mpcd does not serve, and what must never appear is
// a stub that reports success: a payment rail that says "ok" without moving
// money is worse than one that says "no".
func TestSpendingMethodsRefuse(t *testing.T) {
	s := &fakeMPCD{reply: `{}`}
	srv := s.serve(t)
	defer srv.Close()
	p := processorFor(srv.URL)
	ctx := context.Background()

	if _, err := p.Charge(ctx, chargeReq()); err == nil {
		t.Error("Charge reported success")
	}
	if _, err := p.Authorize(ctx, chargeReq()); err == nil {
		t.Error("Authorize reported success")
	}
	if _, err := p.GetTransaction(ctx, "tx"); err == nil {
		t.Error("GetTransaction reported success")
	}
	if _, err := p.GetBalance(ctx, "0xabc", "ethereum"); err == nil {
		t.Error("GetBalance reported success — it used to answer zero for every funded address")
	}
	// None of them may have touched the network.
	if s.path != "" {
		t.Errorf("a refusing method still called mpcd at %q", s.path)
	}
}

func chargeReq() processor.PaymentRequest {
	return processor.PaymentRequest{
		Amount:     currency.Cents(1000),
		Currency:   currency.Type("usdc"),
		CustomerID: "acme",
		Address:    "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
		Chain:      "ethereum",
	}
}
