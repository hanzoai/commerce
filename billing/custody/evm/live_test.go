package evm

import (
	"context"
	"encoding/hex"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/custody"
	"github.com/hanzoai/commerce/billing/husdindex"
)

// A LIVE read against Ethereum mainnet. It NEVER broadcasts.
//
// The unit tests prove our bytes match an independent implementation's. Only
// this proves the things a fixture cannot: that a real node answers the calls a
// sweep makes, in the shapes we decode, and — the part that matters most — that
// the DEPLOYED USDC contract accepts our transfer calldata. If the selector
// were wrong by a nibble or a word were padded on the wrong side,
// eth_estimateGas reverts here and passes in every fixture we could write.
//
//	EVM_LIVE=1 go test ./billing/custody/evm/ -run TestLive -v
//
// There is deliberately no broadcast in this file and there must never be one.
// A signed transaction sent from a test is money moving, and the vector key
// this package uses is public — anyone can send funds to its address, so a
// "harmless" replay could stop being harmless without warning.
const (
	liveRPC  = "https://ethereum-rpc.publicnode.com"
	liveUSDC = "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"
	// A long-lived exchange hot wallet: it holds USDC and ETH, so gas can be
	// estimated for a transfer that would really succeed. If it ever empties,
	// the test says so and skips rather than failing — it is evidence about the
	// world, not about our code.
	liveHolder = "0xF977814e90dA44bFA03b6295A0616a897441aceC"
	liveDest   = "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"
)

func liveCtx(t *testing.T) context.Context {
	t.Helper()
	if os.Getenv("EVM_LIVE") == "" {
		t.Skip("set EVM_LIVE=1 to read Ethereum mainnet")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// TestLiveReadsTheChain covers every call a sweep makes except the broadcast.
func TestLiveReadsTheChain(t *testing.T) {
	ctx := liveCtx(t)

	c, err := New(ctx, custody.Ethereum, liveRPC)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.ChainID().Int64() != 1 {
		t.Fatalf("chain id %s, want 1 for Ethereum mainnet", c.ChainID())
	}
	t.Logf("chain id: %s", c.ChainID())

	nonce, err := c.rpc.Nonce(ctx, liveHolder)
	if err != nil {
		t.Fatalf("Nonce: %v", err)
	}
	t.Logf("nonce of %s: %d", liveHolder, nonce)

	base, err := c.rpc.BaseFee(ctx)
	if err != nil {
		t.Fatalf("BaseFee: %v", err)
	}
	if base.Sign() <= 0 {
		t.Errorf("base fee is %s; mainnet has had a positive base fee since London", base)
	}
	t.Logf("pending base fee: %s wei", base)

	tip, err := c.rpc.Tip(ctx)
	if err != nil {
		t.Fatalf("Tip: %v", err)
	}
	t.Logf("suggested tip: %s wei", tip)

	native, err := c.rpc.NativeBalance(ctx, liveHolder)
	if err != nil {
		t.Fatalf("NativeBalance: %v", err)
	}
	t.Logf("native balance: %s wei", native)
}

// TestLiveContractAcceptsOurCalldata is the assertion that a fixture cannot
// make.
//
// eth_estimateGas EXECUTES the call against real state. A wrong selector, a
// right-padded address, a mis-sized word — every ABI mistake that a hand-rolled
// encoder can make — reverts here. Passing means the deployed USDC contract
// read our bytes as transfer(to, amount) and was willing to run it.
func TestLiveContractAcceptsOurCalldata(t *testing.T) {
	ctx := liveCtx(t)
	rpc := husdindex.NewClient(liveRPC, liveUSDC)

	held, err := rpc.BalanceOf(ctx, liveHolder)
	if err != nil {
		t.Fatalf("BalanceOf: %v", err)
	}
	t.Logf("%s holds %s USDC base units", liveHolder, held)
	if held.Sign() == 0 {
		t.Skipf("%s no longer holds USDC; nothing about our code is proven or disproven by that", liveHolder)
	}

	dest, err := parseAddr(liveDest)
	if err != nil {
		t.Fatal(err)
	}
	// One whole USDC, well under the balance.
	data := transferCall(dest, big.NewInt(1_000_000))
	gas, err := rpc.EstimateGas(ctx, liveHolder, liveUSDC, nil, data)
	if err != nil {
		t.Fatalf("the deployed USDC contract rejected our transfer calldata %s: %v", hex.EncodeToString(data), err)
	}
	t.Logf("USDC accepted our calldata; estimated gas %d", gas)
	if gas < 21_000 {
		t.Errorf("estimate %d is below the intrinsic cost of any transaction", gas)
	}

	// And the negative control: a transfer larger than the balance MUST be
	// refused by the contract. If this passed, the estimate above would be
	// telling us nothing about whether the contract understood us.
	tooMuch := new(big.Int).Mul(held, big.NewInt(1_000_000))
	if _, err := rpc.EstimateGas(ctx, liveHolder, liveUSDC, nil, transferCall(dest, tooMuch)); err == nil {
		t.Error("the contract accepted a transfer of more than the balance; the estimate proves nothing")
	}
}

// TestLiveDraftsAgainstRealState builds a complete unsigned sweep from a real
// funded address and checks it is coherent. It stops one step short of signing,
// because the fleet holds no key for this address and must not be asked for one.
func TestLiveDraftsAgainstRealState(t *testing.T) {
	ctx := liveCtx(t)

	c, err := New(ctx, custody.Ethereum, liveRPC)
	if err != nil {
		t.Fatal(err)
	}
	d, err := c.Draft(ctx, custody.Transfer{
		OrgID: "live", WalletID: "none", From: liveHolder, To: liveDest,
		Token: liveUSDC, Amount: big.NewInt(1_000_000),
	})
	if err != nil {
		// ErrNoFee here would be real information about the address, not a bug.
		var noFee *custody.ErrNoFee
		if asErrNoFee(err, &noFee) {
			t.Skipf("%s cannot pay its own gas right now: %v", liveHolder, err)
		}
		t.Fatalf("Draft against live state: %v", err)
	}
	if len(d.Digests) != 1 || len(d.Digests[0]) != 32 {
		t.Fatalf("want one 32-byte digest, got %d of length %d", len(d.Digests), len(d.Digests[0]))
	}
	t.Logf("drafted a live sweep; digest to sign: %s", hex.EncodeToString(d.Digests[0]))

	// Seal must refuse the all-zero signature even here — the check is not a
	// fixture-only affordance.
	if _, err := d.Seal([][]byte{make([]byte, 64)}); err == nil {
		t.Fatal("Seal assembled a live transaction from an empty signature")
	} else if !strings.Contains(err.Error(), "does not belong to custody address") {
		t.Fatalf("unexpected refusal: %v", err)
	}
}

// TestLiveBaseChainID checks a second network, because the chain id is what
// stops a signature made for one from being valid on the other.
func TestLiveBaseChainID(t *testing.T) {
	ctx := liveCtx(t)
	c, err := New(ctx, custody.Base, "https://base-rpc.publicnode.com")
	if err != nil {
		t.Skipf("Base RPC unreachable: %v", err)
	}
	if c.ChainID().Int64() != 8453 {
		t.Fatalf("Base reported chain id %s, want 8453", c.ChainID())
	}
	t.Logf("Base chain id: %s", c.ChainID())
}
