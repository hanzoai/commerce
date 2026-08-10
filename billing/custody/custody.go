// Package custody spends money OUT of an address whose key the MPC fleet holds.
//
// billing/depositwatch answers "did money arrive"; this answers "can we move
// it". Until this package existed the answer was no on every chain: the rail
// minted per-payer custody addresses, credited deposits against them, and had
// no path to the coins at all. What stood in for one was a call from the MPC
// payment processor to POST {mpcEndpoint}/api/v1/transactions, asking the
// SIGNER to build, sign and broadcast a chain transaction. mpcd serves no such
// route — it answers 404, and its entire internal surface is /healthz, /keys,
// /backup, /keygen and /sign — and it should not: it is a threshold signer.
// Handing it a chain, a token and an amount would put chain semantics inside
// the one component that must only ever see a digest.
//
// So the split here is the whole design. A CHAIN knows how to read its own
// state, encode a transfer, and say exactly which bytes must be signed. A
// SIGNER turns a digest into a signature and knows nothing else. Sweep is the
// twenty lines that join them. Nothing about Bitcoin's inputs or Ethereum's
// nonce is visible to the signer, and nothing about threshold cryptography is
// visible to a chain.
package custody

import (
	"context"
	"errors"
	"fmt"
	"math/big"
)

// Curve is WHICH key of an MPC wallet signs.
//
// One keygen mints two keys — secp256k1 (CGGMP21) and Ed25519 (FROST) — and
// every address the fleet hands out derives from one or the other. Signing with
// the wrong one produces a signature that verifies against a key which does not
// own the coins: not an error any chain reports usefully, just a transaction
// that is invalid, or an address nobody can ever spend from.
//
// It is never a parameter at a call site. Callers name a Network; the curve
// follows from it. That is not decoration — it is how the fleet's own worst
// recorded defect is kept out of this package. In luxfi/mpc, a FROST keygen
// called the Taproot entry point (BIP-340 over secp256k1) where it meant
// Ed25519: the key is 32 bytes either way, so the length matched, the base58
// address parsed, and the funds simply never moved. Anything that lets a caller
// say "curve" invites that mistake back.
type Curve string

const (
	// Secp256k1 signs for Bitcoin, every EVM chain, and XRPL. XRPL is on this
	// curve here — not Ed25519, which the ledger also supports and which is
	// most people's first guess.
	Secp256k1 Curve = "secp256k1"
	// Ed25519 signs for Solana and TON.
	Ed25519 Curve = "ed25519"
)

// Network is the code the signer resolves a curve from.
//
// It is a NETWORK and not a curve because that is mpcd's contract, and mpcd is
// right about it: cmd/mpcd/main.go says "the request names a NETWORK, not a
// curve … so that every caller cannot get it independently wrong — and so a
// caller cannot ask for a Solana signature under a secp256k1 key by
// mislabelling the request." The daemon owns the mapping; we send the truth
// about which chain we are on and let it decide.
//
// The values are commerce's own chain names, which mpcd accepts through its
// lowercase alias table. Sending our name rather than translating to a canonical
// code keeps one fewer table in the world, and it keeps the audit record on the
// signer side saying what we actually did.
type Network string

const (
	Ethereum Network = "ethereum"
	Base     Network = "base"
	Polygon  Network = "polygon"
	Arbitrum Network = "arbitrum"
	Optimism Network = "optimism"
	BSC      Network = "bsc"
	Lux      Network = "lux"
	Bitcoin  Network = "bitcoin"
	Solana   Network = "solana"
	TON      Network = "ton"
	XRPL     Network = "xrpl"
)

// curves is the ONE place a network's curve is written down on this side.
//
// It is a copy of a table that lives authoritatively in the signer
// (luxfi/mpc pkg/types networkKeyType), and copies drift, so it is used for
// exactly two things and never for a third: choosing how to PARSE the
// signature that comes back (65 bytes recoverable, or 64 bytes raw), and
// refusing a network we do not understand before dialling out. It is never
// sent. If the two tables ever disagree the sweep still cannot go wrong,
// because Draft.Seal proves the returned signature belongs to the address being
// spent before it will assemble anything.
//
// A network absent here is refused. Two of commerce's mint chains are
// deliberately absent because the SIGNER does not know them either: avalanche
// and zoo are in thirdparty/mpc's mintChains, so the rail will hand out custody
// addresses on both, but mpcd's alias table has no entry for either name and
// answers 400 "unknown network". An address that can be minted and never signed
// for is a one-way door, and it is better to find that here than at a sweep.
var curves = map[Network]Curve{
	Ethereum: Secp256k1,
	Base:     Secp256k1,
	Polygon:  Secp256k1,
	Arbitrum: Secp256k1,
	Optimism: Secp256k1,
	BSC:      Secp256k1,
	Lux:      Secp256k1,
	Bitcoin:  Secp256k1,
	XRPL:     Secp256k1,
	Solana:   Ed25519,
	TON:      Ed25519,
}

// Curve reports which of a wallet's keys signs on n, and whether n is a network
// this rail can sign on at all.
func (n Network) Curve() (Curve, bool) {
	c, ok := curves[n]
	return c, ok
}

// Signer is the MPC fleet, reduced to the only thing it does: turn a digest
// into a signature under one wallet's key.
//
// It takes a DIGEST and not a message, because who hashes is not a detail. Each
// chain hashes differently — keccak256 over an RLP payload on EVM, double
// SHA-256 over a BIP-143 preimage on Bitcoin, not at all on Solana because
// Ed25519 hashes internally — and a signer that hashed on the caller's behalf
// would have to know which chain it was serving. That knowledge is exactly what
// this interface exists to keep out. mpcd agrees: it hex-decodes payload_hash
// and signs those bytes untouched.
type Signer interface {
	// Sign returns the signature over digest by walletID's key on n.
	//
	// On a secp256k1 network the result is 65 bytes, r‖s‖v, with s already
	// normalised into the lower half of the curve order (EIP-2) and v being 0
	// or 1. On an Ed25519 network it is the raw 64-byte RFC 8032 signature.
	Sign(ctx context.Context, orgID, walletID string, n Network, digest []byte) ([]byte, error)
}

// Transfer is one movement of value out of custody.
type Transfer struct {
	// OrgID is the tenant whose custody this is. The signer requires it and
	// scopes the wallet by it, so a sweep that cannot name the org cannot be
	// signed.
	OrgID string

	// WalletID is the signer's own handle for the key behind From. It is
	// recorded on the deposit intent at mint time
	// (cryptopaymentintent.WalletID) precisely so a sweep can name it later.
	WalletID string

	// From is the custody address being spent. It is not merely informational:
	// every chain here proves the signature it got belongs to this address
	// before it will assemble a transaction, so a mismatched WalletID/From pair
	// fails loudly instead of broadcasting a spend from somewhere unintended.
	From string

	// To is where the money goes — the treasury.
	To string

	// Token is the asset's on-chain identity: an ERC-20 contract, an SPL mint,
	// a jetton master. Empty means the chain's native coin.
	Token string

	// Amount is in the asset's base units. Nil means "everything spendable",
	// which each chain computes for itself: an ERC-20 balance in full, a native
	// balance less the fee it takes to move it.
	Amount *big.Int
}

// Draft is an unsigned transfer together with EXACTLY the digests its signer
// must cover.
//
// Digests is a slice and not a single value because Bitcoin makes it one:
// spending three outputs needs three signatures over three different preimages.
// Every account-model chain returns one. Writing the plural shape once means
// Sweep never grows a Bitcoin special case, and it means a chain cannot quietly
// sign one input and leave the rest unsigned.
type Draft struct {
	// Digests are the bytes to sign, in the order Seal expects them back.
	Digests [][]byte

	// Seal assembles the broadcastable transaction from the signatures.
	//
	// It is the last place a mistake can be caught, so it is where the
	// signatures are CHECKED: every implementation proves each signature
	// belongs to the key that owns Transfer.From — by public-key recovery where
	// the curve allows it, by verification against the address's own key where
	// it does not — and returns an error rather than bytes when it does not.
	//
	// This is the property that makes a wrong curve, a wrong wallet id and a
	// confused signer all fail in the same safe place. A sweep that broadcast
	// an unverified signature would spend a fee to learn what a microsecond of
	// arithmetic could have told it, and on a UTXO chain a malformed spend can
	// send change somewhere unrecoverable.
	Seal func(sigs [][]byte) ([]byte, error)
}

// Chain builds and broadcasts on one network.
type Chain interface {
	// Network is the code the signer resolves a curve from. Fixed per chain,
	// which is what keeps it out of callers' hands.
	Network() Network

	// Draft reads whatever chain state a transfer needs — nonce, sequence,
	// blockhash, unspent outputs, fee rate — and returns the unsigned transfer.
	Draft(ctx context.Context, t Transfer) (*Draft, error)

	// Broadcast submits assembled bytes and returns the chain's id for them.
	Broadcast(ctx context.Context, raw []byte) (string, error)
}

// ErrNoFee reports that the address being swept cannot pay for its own
// transfer: it holds the asset but not the coin the chain charges in.
//
// This is the ordinary state of an EVM deposit address, not an exotic failure.
// A payer who sends USDC to a freshly minted address sends no ETH with it, so
// the address holds a token it cannot move, and somebody has to send it gas
// first. Every EVM sweep of a token begins here the first time.
//
// It is a typed error because the caller's response is specific and automatable
// — fund the address, then sweep — and because the alternative shape, letting
// the transaction go out and fail, spends real money to produce a worse message.
type ErrNoFee struct {
	Network Network
	Addr    string
	Have    *big.Int
	Need    *big.Int
}

func (e *ErrNoFee) Error() string {
	return fmt.Sprintf("custody: %s address %s holds %s of the native coin but needs %s to pay for this transfer; fund it before sweeping",
		e.Network, e.Addr, e.Have, e.Need)
}

// ErrEmpty reports that there is nothing to sweep. It is not a failure — a
// scheduler that walks every known address will meet it constantly — so it is
// distinguishable with errors.Is rather than by matching a string.
var ErrEmpty = errors.New("custody: nothing to sweep")

// Sweep moves Transfer out of custody and returns the chain's transaction id.
//
// This is the only orchestration in the package and it is deliberately this
// short. Read state, sign each digest, assemble, broadcast. Everything that
// varies by chain is behind Chain; everything that varies by signer is behind
// Signer; the ORDER is the same everywhere, so it is written once.
func Sweep(ctx context.Context, c Chain, s Signer, t Transfer) (string, error) {
	if t.OrgID == "" || t.WalletID == "" || t.From == "" || t.To == "" {
		return "", fmt.Errorf("custody: sweep needs orgID, walletID, from and to (got %q, %q, %q, %q)",
			t.OrgID, t.WalletID, t.From, t.To)
	}
	if t.Amount != nil && t.Amount.Sign() <= 0 {
		return "", fmt.Errorf("custody: sweep amount must be positive, or nil for everything; got %s", t.Amount)
	}
	n := c.Network()
	if _, ok := n.Curve(); !ok {
		return "", fmt.Errorf("custody: %q is not a network this rail can sign on", n)
	}

	d, err := c.Draft(ctx, t)
	if err != nil {
		return "", err
	}
	if len(d.Digests) == 0 {
		return "", errors.New("custody: chain drafted a transfer with nothing to sign")
	}

	sigs := make([][]byte, len(d.Digests))
	for i, digest := range d.Digests {
		sig, err := s.Sign(ctx, t.OrgID, t.WalletID, n, digest)
		if err != nil {
			// Naming the index matters on Bitcoin, where one failure among
			// several inputs is otherwise indistinguishable from all of them.
			return "", fmt.Errorf("custody: signing digest %d of %d: %w", i+1, len(d.Digests), err)
		}
		sigs[i] = sig
	}

	raw, err := d.Seal(sigs)
	if err != nil {
		return "", err
	}
	return c.Broadcast(ctx, raw)
}
