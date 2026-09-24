package custody

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"
)

// TestCurveTable pins the mapping that decides which of a wallet's two keys
// signs.
//
// The values are asserted one by one rather than looped, because the point of
// this test is to be READ. XRPL on secp256k1 is the entry most likely to be
// "corrected" by someone who knows the ledger also supports Ed25519, and
// Solana on Ed25519 is the one whose violation is famous here: in luxfi/mpc a
// FROST keygen once called the secp256k1 Taproot entry point where it meant
// Ed25519, producing 32-byte keys that base58-encoded into plausible Solana
// addresses nobody could ever spend from.
func TestCurveTable(t *testing.T) {
	for n, want := range map[Network]Curve{
		Ethereum: Secp256k1,
		Base:     Secp256k1,
		Polygon:  Secp256k1,
		Arbitrum: Secp256k1,
		Optimism: Secp256k1,
		BSC:      Secp256k1,
		Lux:      Secp256k1,
		Bitcoin:  Secp256k1,
		XRPL:     Secp256k1, // NOT Ed25519 on this rail
		Solana:   Ed25519,
		TON:      Ed25519,
	} {
		got, ok := n.Curve()
		if !ok {
			t.Errorf("%s is not a known network", n)
			continue
		}
		if got != want {
			t.Errorf("%s signs on %s, want %s", n, got, want)
		}
	}
}

// TestUnknownNetworksAreRefused covers the two chains commerce can mint an
// address for and the signer cannot sign for.
//
// thirdparty/mpc's mintChains offers avalanche and zoo, so the rail will hand a
// payer a custody address on either; mpcd's alias table has no entry for either
// name and answers 400 "unknown network". Refusing them here means a sweep
// fails before it dials rather than after, and it means this test fails the day
// someone adds them to one table and not the other.
func TestUnknownNetworksAreRefused(t *testing.T) {
	for _, n := range []Network{"avalanche", "zoo", "cardano", "", "ETHEREUM", "secp256k1", "ed25519"} {
		if _, ok := n.Curve(); ok {
			t.Errorf("%q is treated as a signable network", n)
		}
	}
}

// --- fakes ---

type fakeChain struct {
	network   Network
	digests   [][]byte
	sealed    [][]byte
	sealErr   error
	draftErr  error
	broadcast string
	raw       []byte
}

func (f *fakeChain) Network() Network { return f.network }

func (f *fakeChain) Draft(context.Context, Transfer) (*Draft, error) {
	if f.draftErr != nil {
		return nil, f.draftErr
	}
	return &Draft{
		Digests: f.digests,
		Seal: func(sigs [][]byte) ([]byte, error) {
			f.sealed = sigs
			if f.sealErr != nil {
				return nil, f.sealErr
			}
			return []byte("assembled"), nil
		},
	}, nil
}

func (f *fakeChain) Broadcast(_ context.Context, raw []byte) (string, error) {
	f.raw = raw
	return f.broadcast, nil
}

type fakeSigner struct {
	seen []string // "org/wallet/network/digest"
	err  error
	at   int // index at which to fail
}

func (s *fakeSigner) Sign(_ context.Context, org, wallet string, n Network, digest []byte) ([]byte, error) {
	i := len(s.seen)
	s.seen = append(s.seen, org+"/"+wallet+"/"+string(n)+"/"+string(digest))
	if s.err != nil && i == s.at {
		return nil, s.err
	}
	return append([]byte("sig-"), digest...), nil
}

func goodTransfer() Transfer {
	return Transfer{OrgID: "acme", WalletID: "w1", From: "0xfrom", To: "0xto"}
}

// TestSweepSignsEveryDigestInOrder is the Bitcoin shape, checked on the generic
// path: a chain that needs three signatures gets three, over the right bytes,
// in the order Seal expects them back.
//
// A sweep that signed only the first input would produce a transaction that is
// simply invalid; one that signed them out of order would produce one that is
// invalid in a way that is much harder to see.
func TestSweepSignsEveryDigestInOrder(t *testing.T) {
	c := &fakeChain{
		network:   Bitcoin,
		digests:   [][]byte{[]byte("in0"), []byte("in1"), []byte("in2")},
		broadcast: "txid",
	}
	s := &fakeSigner{}

	id, err := Sweep(context.Background(), c, s, goodTransfer())
	if err != nil {
		t.Fatal(err)
	}
	if id != "txid" {
		t.Fatalf("Sweep returned %q, want the chain's id", id)
	}
	want := []string{"acme/w1/bitcoin/in0", "acme/w1/bitcoin/in1", "acme/w1/bitcoin/in2"}
	if len(s.seen) != len(want) {
		t.Fatalf("signed %d digests, want %d", len(s.seen), len(want))
	}
	for i := range want {
		if s.seen[i] != want[i] {
			t.Errorf("digest %d: signed %q, want %q", i, s.seen[i], want[i])
		}
	}
	for i, sig := range c.sealed {
		if string(sig) != "sig-"+string(c.digests[i]) {
			t.Errorf("Seal got signature %d = %q, out of order", i, sig)
		}
	}
	if string(c.raw) != "assembled" {
		t.Errorf("broadcast %q, want the sealed bytes", c.raw)
	}
}

// TestSweepNamesTheFailingInput matters on a UTXO chain, where one failure
// among several is otherwise indistinguishable from all of them.
func TestSweepNamesTheFailingInput(t *testing.T) {
	c := &fakeChain{network: Bitcoin, digests: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}
	s := &fakeSigner{err: errors.New("quorum lost"), at: 1}

	_, err := Sweep(context.Background(), c, s, goodTransfer())
	if err == nil {
		t.Fatal("Sweep succeeded with a failing signer")
	}
	if !strings.Contains(err.Error(), "digest 2 of 3") {
		t.Fatalf("error should name which input failed, got: %v", err)
	}
	if c.sealed != nil {
		t.Fatal("Seal ran despite a signing failure")
	}
}

// TestSweepDoesNotBroadcastWhenSealRefuses is the property the whole design
// hangs on: Seal is where a signature is proven to belong to the address being
// spent, so a refusal there must never reach the network.
func TestSweepDoesNotBroadcastWhenSealRefuses(t *testing.T) {
	c := &fakeChain{network: Ethereum, digests: [][]byte{[]byte("d")}, sealErr: errors.New("foreign signature")}
	_, err := Sweep(context.Background(), c, &fakeSigner{}, goodTransfer())
	if err == nil {
		t.Fatal("Sweep succeeded despite Seal refusing")
	}
	if c.raw != nil {
		t.Fatal("broadcast something Seal had refused to assemble")
	}
}

func TestSweepValidatesTheTransfer(t *testing.T) {
	c := &fakeChain{network: Ethereum, digests: [][]byte{[]byte("d")}}
	for name, tr := range map[string]Transfer{
		"no org":    {WalletID: "w", From: "a", To: "b"},
		"no wallet": {OrgID: "o", From: "a", To: "b"},
		"no from":   {OrgID: "o", WalletID: "w", To: "b"},
		"no to":     {OrgID: "o", WalletID: "w", From: "a"},
		"zero":      {OrgID: "o", WalletID: "w", From: "a", To: "b", Amount: big.NewInt(0)},
		"negative":  {OrgID: "o", WalletID: "w", From: "a", To: "b", Amount: big.NewInt(-1)},
	} {
		if _, err := Sweep(context.Background(), c, &fakeSigner{}, tr); err == nil {
			t.Errorf("%s: Sweep accepted it", name)
		}
	}
}

// TestSweepRefusesUnsignableNetwork stops before dialling out. A chain object
// claiming a network the signer does not know would otherwise spend a round
// trip to learn it, and would do so once per address in a sweep loop.
func TestSweepRefusesUnsignableNetwork(t *testing.T) {
	c := &fakeChain{network: "avalanche", digests: [][]byte{[]byte("d")}}
	s := &fakeSigner{}
	if _, err := Sweep(context.Background(), c, s, goodTransfer()); err == nil {
		t.Fatal("Sweep proceeded on a network the signer cannot sign for")
	}
	if len(s.seen) != 0 {
		t.Fatal("dialled the signer for an unsignable network")
	}
}

func TestSweepRefusesAnEmptyDraft(t *testing.T) {
	c := &fakeChain{network: Ethereum, digests: nil}
	if _, err := Sweep(context.Background(), c, &fakeSigner{}, goodTransfer()); err == nil {
		t.Fatal("Sweep accepted a draft with nothing to sign")
	}
}

func TestErrNoFeeSaysWhatToSend(t *testing.T) {
	e := &ErrNoFee{Network: Ethereum, Addr: "0xabc", Have: big.NewInt(5), Need: big.NewInt(100)}
	msg := e.Error()
	for _, want := range []string{"ethereum", "0xabc", "5", "100", "fund it"} {
		if !strings.Contains(msg, want) {
			t.Errorf("ErrNoFee message is missing %q: %s", want, msg)
		}
	}
}
