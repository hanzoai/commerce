package mpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/hanzoai/commerce/billing/custody"
)

// signReq is mpcd's POST /sign body (luxfi/mpc cmd/mpcd/main.go).
//
// There is no curve field and there must not be one. The daemon derives the
// curve from network, deliberately, "so that every caller cannot get it
// independently wrong — and so a caller cannot ask for a Solana signature under
// a secp256k1 key by mislabelling the request". We send the chain we are
// actually on and let it choose.
//
// chain_id is omitted: the daemon accepts it, folds it into the idempotency
// hash and routes on nothing, so sending a number that means one thing to us
// and nothing to it would only make two records disagree.
type signReq struct {
	OrgID          string `json:"org_id"`
	WalletID       string `json:"wallet_id"`
	Network        string `json:"network"`
	PayloadHash    string `json:"payload_hash"`
	IdempotencyKey string `json:"idempotency_key"`
}

// signResp is mpcd's reply.
//
// Status is the field that matters and the one an ordinary HTTP client would
// miss: a signing FAILURE comes back as 200 with status "rejected", because the
// daemon reserves 503 for having no quorum. Treating 200 as success would read
// an empty signature as a good one.
type signResp struct {
	SessionID string `json:"session_id"`
	Signature string `json:"signature"`
	Status    string `json:"status"`
	Error     string `json:"error"`
}

// Sign implements custody.Signer against mpcd's /sign.
//
// It is the whole of commerce's spending relationship with the signer: hand
// over a digest, get back a signature. Nothing about the transaction the digest
// came from crosses this line, which is why the dead POST /api/v1/transactions
// path it replaces could not be repointed — that call asked the signer to build
// and broadcast a chain transaction, and no endpoint on mpcd does that or
// should.
func (mp *MPCProcessor) Sign(ctx context.Context, orgID, walletID string, n custody.Network, digest []byte) ([]byte, error) {
	curve, ok := n.Curve()
	if !ok {
		return nil, fmt.Errorf("mpc: %q is not a network this rail signs on", n)
	}
	if orgID == "" || walletID == "" {
		return nil, fmt.Errorf("mpc: sign needs org_id and wallet_id (got %q, %q)", orgID, walletID)
	}
	// The daemon enforces this too, but failing here names the caller's bug
	// instead of returning someone else's 400 several layers up.
	if curve == custody.Secp256k1 && len(digest) != 32 {
		return nil, fmt.Errorf("mpc: %s needs a 32-byte digest, got %d bytes", n, len(digest))
	}
	if len(digest) == 0 {
		return nil, fmt.Errorf("mpc: nothing to sign")
	}

	var resp signResp
	err := mp.http.doJSON(ctx, http.MethodPost, mp.mpcEndpoint+"/sign", &signReq{
		OrgID:          orgID,
		WalletID:       walletID,
		Network:        string(n),
		PayloadHash:    hex.EncodeToString(digest),
		IdempotencyKey: idemKey(orgID, walletID, n, digest),
	}, &resp)
	if err != nil {
		return nil, fmt.Errorf("mpc: sign request: %w", err)
	}
	if resp.Status != "signed" {
		reason := resp.Error
		if reason == "" {
			reason = "no reason given"
		}
		return nil, fmt.Errorf("mpc: signer returned status %q for session %s: %s", resp.Status, resp.SessionID, reason)
	}

	sig, err := hex.DecodeString(strings.TrimPrefix(resp.Signature, "0x"))
	if err != nil {
		return nil, fmt.Errorf("mpc: signature is not hex: %w", err)
	}
	// Length is the one property checkable without the transaction, and a wrong
	// one means the daemon signed on a curve we did not expect. Refusing here
	// keeps a 64-byte Ed25519 signature from being read as a truncated
	// recoverable one, and vice versa.
	want := 65
	if curve == custody.Ed25519 {
		want = 64
	}
	if len(sig) != want {
		return nil, fmt.Errorf("mpc: %s signature should be %d bytes on %s, got %d", n, want, curve, len(sig))
	}
	return sig, nil
}

// idemKey makes a retry free.
//
// mpcd keys signing on it: the same key with the same content returns the
// stored signature without running a second ceremony, and the same key with
// DIFFERENT content is refused outright with 409 rather than signed. Deriving
// the key from the content itself gets both halves right — a resumed sweep
// re-derives the identical key and collects the signature it already paid for,
// and two different transfers can never collide onto one.
//
// A random key would break exactly this: a timeout would leave a signature the
// caller never saw and cannot ask for again, and the next attempt would sign a
// second, different transaction over the same coins.
func idemKey(orgID, walletID string, n custody.Network, digest []byte) string {
	h := sha256.New()
	// Length-prefixed so no two different tuples can flatten to the same bytes
	// ("ab"+"c" and "a"+"bc" are the classic way to lose that).
	for _, part := range []string{"custody-sweep-v1", orgID, walletID, string(n)} {
		fmt.Fprintf(h, "%d:%s", len(part), part)
	}
	h.Write(digest)
	return hex.EncodeToString(h.Sum(nil))
}

// The processor is the rail's one mpcd client, so it is also its signer.
var _ custody.Signer = (*MPCProcessor)(nil)
