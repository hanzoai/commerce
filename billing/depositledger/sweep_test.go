package depositledger

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/custody"
	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/models/cryptopaymentintent"
	"github.com/hanzoai/commerce/util/test/ae"
)

// These are REAL-datastore tests for the same reason the credit ones are: the
// thing being proven is that the sweep walks the addresses the rail actually
// minted, and only a real store holds those.
//
// What is faked is the CHAIN and the SIGNER, deliberately and completely. Every
// property below is about which addresses are attempted and with what, and none
// of it depends on real cryptography — the cryptography is proven against
// mainnet vectors in billing/custody/{evm,bitcoin,solana}, where a fake would
// prove nothing.

// sweepChain records the transfers it was asked to draft and answers each one
// however the test said to.
type sweepChain struct {
	network custody.Network
	drafted []custody.Transfer
	// answer decides what Draft does with the nth transfer, by From address.
	empty map[string]bool
	fail  map[string]error
}

func (c *sweepChain) Network() custody.Network { return c.network }

func (c *sweepChain) Draft(_ context.Context, t custody.Transfer) (*custody.Draft, error) {
	c.drafted = append(c.drafted, t)
	if c.empty[strings.ToLower(t.From)] {
		return nil, custody.ErrEmpty
	}
	if err := c.fail[strings.ToLower(t.From)]; err != nil {
		return nil, err
	}
	return &custody.Draft{
		Digests: [][]byte{make([]byte, 32)},
		Seal:    func(sigs [][]byte) ([]byte, error) { return []byte("raw"), nil },
	}, nil
}

func (c *sweepChain) Broadcast(context.Context, []byte) (string, error) { return "0xtx", nil }

// sweepSigner records what it was asked to sign for.
type sweepSigner struct{ wallets []string }

func (s *sweepSigner) Sign(_ context.Context, _, walletID string, _ custody.Network, _ []byte) ([]byte, error) {
	s.wallets = append(s.wallets, walletID)
	return make([]byte, 65), nil
}

// mint persists a minted deposit intent the way api/billing.CreateCryptoDeposit
// does — address AND wallet id, which are the two halves a sweep needs.
func mint(t *testing.T, org, subject, addr, wallet string) {
	t.Helper()
	in := cryptopaymentintent.New(orgDB(org))
	in.Currency = "usd"
	in.Chain = cryptopaymentintent.Ethereum
	in.Token = "usdc"
	in.DepositAddress = addr
	in.WalletID = wallet
	in.CustomerRef = subject
	in.Status = cryptopaymentintent.Pending
	in.ExpiresAt = time.Now().Add(24 * time.Hour)
	in.Defaults()
	if err := in.Create(); err != nil {
		t.Fatalf("mint intent: %v", err)
	}
}

func usdcOnEthereum() depositwatch.Asset {
	return depositwatch.Asset{Chain: "ethereum", Token: "usdc", Contract: testContract, RPCURL: "http://rpc.invalid"}
}

// TestSweepEmptiesEveryMintedAddress is the whole point of the command: money
// arrives at N per-payer addresses and has to end up in one place.
func TestSweepEmptiesEveryMintedAddress(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	seedOrg(t, "acme")

	mint(t, "acme", "acme/alice", "0x1111111111111111111111111111111111111111", "w-alice")
	mint(t, "acme", "acme/bob", "0x2222222222222222222222222222222222222222", "w-bob")

	chain := &sweepChain{network: custody.Ethereum}
	signer := &sweepSigner{}
	const treasury = "0x9999999999999999999999999999999999999999"

	got, err := Sweep(context.Background(), chain, signer, usdcOnEthereum(), treasury)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("swept %d addresses, want 2: %+v", len(got), got)
	}
	for _, r := range got {
		if r.Err != nil {
			t.Errorf("%s: %v", r.Address, r.Err)
		}
		if r.TxID != "0xtx" {
			t.Errorf("%s returned txid %q, want the chain's id", r.Address, r.TxID)
		}
	}

	// Each address is spent to the treasury, under ITS OWN wallet, for the
	// configured token — and never for an amount, because "everything spendable"
	// is the chain's arithmetic and not the operator's.
	for _, d := range chain.drafted {
		if d.To != treasury {
			t.Errorf("drafted a transfer to %q, want the treasury %q", d.To, treasury)
		}
		if !strings.EqualFold(d.Token, testContract) {
			t.Errorf("drafted token %q, want the configured contract %q", d.Token, testContract)
		}
		if d.Amount != nil {
			t.Errorf("drafted a fixed amount %s; a sweep names none", d.Amount)
		}
		if d.OrgID != "acme" {
			t.Errorf("drafted under org %q, want acme", d.OrgID)
		}
	}
	if want := []string{"w-alice", "w-bob"}; !sameSet(signer.wallets, want) {
		t.Errorf("signed under wallets %v, want %v — each address must be spent under its own key", signer.wallets, want)
	}
}

// An address holding nothing is the ORDINARY case, not a failure: a sweep walks
// every address ever minted and most are empty. Reporting them would bury the
// rows that matter under hundreds that do not.
func TestSweepIsSilentAboutEmptyAddresses(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	seedOrg(t, "acme")

	mint(t, "acme", "acme/alice", "0x1111111111111111111111111111111111111111", "w-alice")
	mint(t, "acme", "acme/bob", "0x2222222222222222222222222222222222222222", "w-bob")

	chain := &sweepChain{network: custody.Ethereum, empty: map[string]bool{
		"0x1111111111111111111111111111111111111111": true,
	}}

	got, err := Sweep(context.Background(), chain, &sweepSigner{}, usdcOnEthereum(), "0x9999999999999999999999999999999999999999")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(got) != 1 || !strings.EqualFold(got[0].Address, "0x2222222222222222222222222222222222222222") {
		t.Fatalf("results = %+v, want only the funded address", got)
	}
}

// One address that cannot pay its own gas must not stop the rest. ErrNoFee is
// the ordinary FIRST state of every EVM token deposit — the payer sent USDC and
// no ETH with it — so a sweep that aborted on it would collect nothing, ever.
func TestSweepReportsOneFailureAndKeepsGoing(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	seedOrg(t, "acme")

	const broke = "0x1111111111111111111111111111111111111111"
	mint(t, "acme", "acme/alice", broke, "w-alice")
	mint(t, "acme", "acme/bob", "0x2222222222222222222222222222222222222222", "w-bob")

	chain := &sweepChain{network: custody.Ethereum, fail: map[string]error{
		broke: &custody.ErrNoFee{Network: custody.Ethereum, Addr: broke, Have: big.NewInt(0), Need: big.NewInt(21000)},
	}}

	got, err := Sweep(context.Background(), chain, &sweepSigner{}, usdcOnEthereum(), "0x9999999999999999999999999999999999999999")
	if err != nil {
		t.Fatalf("Sweep returned a fatal error for one unfundable address: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("results = %+v, want one failure and one success", got)
	}
	var noFee *custody.ErrNoFee
	for _, r := range got {
		if strings.EqualFold(r.Address, broke) {
			if !errors.As(r.Err, &noFee) {
				t.Errorf("the unfundable address reported %v, want ErrNoFee so the operator knows to fund it", r.Err)
			}
			continue
		}
		if r.Err != nil || r.TxID == "" {
			t.Errorf("the funded address did not sweep: %+v", r)
		}
	}
}

// An intent minted before wallet_id was persisted names an address holding real
// money and no key to move it with. Skipping it silently is how that money would
// stay lost; the row is the only thing that says it needs a human.
func TestSweepReportsAnAddressWithNoWalletID(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	seedOrg(t, "acme")

	const orphan = "0x1111111111111111111111111111111111111111"
	mint(t, "acme", "acme/alice", orphan, "")

	chain := &sweepChain{network: custody.Ethereum}
	got, err := Sweep(context.Background(), chain, &sweepSigner{}, usdcOnEthereum(), "0x9999999999999999999999999999999999999999")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(got) != 1 || got[0].Err == nil {
		t.Fatalf("results = %+v, want the orphaned address reported", got)
	}
	if len(chain.drafted) != 0 {
		t.Error("drafted a transfer for an address with no wallet id; there is no key to sign it")
	}
}

// Two intents naming one address must produce ONE attempt. Two would be two
// spends of the same coins, drafted against identical chain state and therefore
// both signable — the second losing only once the first is mined, after the
// fleet has already signed it.
func TestSweepAttemptsEachAddressOnce(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	seedOrg(t, "acme")

	const shared = "0x1111111111111111111111111111111111111111"
	mint(t, "acme", "acme/alice", shared, "w-alice")
	mint(t, "acme", "acme/alice", shared, "w-alice")

	chain := &sweepChain{network: custody.Ethereum}
	got, err := Sweep(context.Background(), chain, &sweepSigner{}, usdcOnEthereum(), "0x9999999999999999999999999999999999999999")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(chain.drafted) != 1 || len(got) != 1 {
		t.Fatalf("drafted %d transfers for one address (results %d); want exactly one", len(chain.drafted), len(got))
	}
}

// A pooled chain has ONE account for every payer, so there is nothing per-payer
// to walk. Running the loop over it would draft one spend of that balance per
// intent — N conflicting transactions over one pot.
func TestSweepRefusesAPooledAsset(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	pooled := depositwatch.Asset{Chain: "xrpl", Token: "rlusd", PooledAddress: "rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De"}
	if _, err := Sweep(context.Background(), &sweepChain{}, &sweepSigner{}, pooled, "rTreasury"); err == nil {
		t.Fatal("swept a pooled asset per payer")
	}
}

// A destination is never defaulted. There is no address this could fall back to
// that would not be somebody's money going somewhere nobody chose.
func TestSweepRefusesWithoutADestination(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	if _, err := Sweep(context.Background(), &sweepChain{}, &sweepSigner{}, usdcOnEthereum(), ""); err == nil {
		t.Fatal("swept to nowhere")
	}
}

// Chain is exhaustive, not defaulted. TON and XRPL are readable and creditable
// and have no encoder in billing/custody, so they must refuse HERE rather than
// be handed the EVM writer — which would produce a well-formed Ethereum
// transaction spending a TON address that does not exist.
func TestChainRefusesAChainItCannotWrite(t *testing.T) {
	for _, a := range []depositwatch.Asset{
		{Chain: "ton", Token: "usdt", RPCURL: "http://rpc.invalid"},
		{Chain: "xrpl", Token: "rlusd", RPCURL: "http://rpc.invalid"},
	} {
		if _, err := Chain(context.Background(), a); err == nil {
			t.Errorf("%s: Chain returned a writer for a chain custody cannot encode", a.Chain)
		}
	}
}

func sameSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]int{}
	for _, g := range got {
		seen[g]++
	}
	for _, w := range want {
		seen[w]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}
