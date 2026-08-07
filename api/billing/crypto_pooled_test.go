package billing

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/cryptopaymentintent"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The MINT side of the pooled model.
//
// These test the rules and not the handler, deliberately, and the reason is
// written into crypto_options_test.go above: a handler-driven test here passes
// against ANY mutant, because CreateCryptoDeposit answers 503 at its first line
// (cryptoDepositsCanBeCredited) and never reaches the assertion. A test that
// cannot fail is worse than no test, so each rule is exercised where it lives.

const poolAccount = "rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De"

func xrplWatched() []depositwatch.Asset {
	return []depositwatch.Asset{
		{Chain: "xrpl", Token: "rlusd", PooledAddress: poolAccount},
		{Chain: "base", Token: "usdc"},
	}
}

// A pooled chain is offered because its account is CONFIGURED, not because the
// custody signer says it can derive an address there.
//
// This is the case the previous rule got wrong in the other direction. The MPC
// signer answers "no" for xrpl — it derives no XRPL key at all, and its chain
// switch falls through to the EVM address — so deferring to it would hide a
// chain the rail can both receive on and credit. Meanwhile solana must STILL be
// hidden, because there the signer's "no" is the whole truth.
func TestOfferedFrom_APooledChainDoesNotNeedTheCustodySigner(t *testing.T) {
	// The real MPC chain list: no xrpl, no solana.
	mintable := []string{"bitcoin", "ethereum", "polygon", "arbitrum", "optimism", "base", "avalanche", "lux", "zoo", "bsc"}

	chains, tokens := offeredFrom([]depositwatch.Asset{
		{Chain: "xrpl", Token: "rlusd", PooledAddress: poolAccount},
		{Chain: "solana", Token: "usdc"},
		{Chain: "base", Token: "usdc"},
	}, mintable)

	if got, want := chains, []string{"base", "xrpl"}; !eq(got, want) {
		t.Fatalf("chains = %v, want %v — a pooled chain is offerable on its own configuration", got, want)
	}
	if got, want := tokens, []string{"rlusd", "usdc"}; !eq(got, want) {
		t.Fatalf("tokens = %v, want %v", got, want)
	}
}

// The per-payer half of the rule is untouched: a chain the signer cannot mint
// on is still hidden, and nothing about pooling relaxes that.
func TestOfferedFrom_APerPayerChainStillNeedsTheCustodySigner(t *testing.T) {
	chains, tokens := offeredFrom([]depositwatch.Asset{{Chain: "solana", Token: "usdc"}},
		[]string{"ethereum", "base"})
	if len(chains) != 0 || len(tokens) != 0 {
		t.Fatalf("an unmintable per-payer chain is offered: chains=%v tokens=%v", chains, tokens)
	}
}

// Nothing watched, nothing offered — including pooled chains. A pooled chain
// that is not configured must not slip past the empty-menu rule.
func TestOfferedFrom_NothingWatchedOffersNothing(t *testing.T) {
	chains, tokens := offeredFrom(nil, []string{"xrpl", "ethereum"})
	if len(chains) != 0 || len(tokens) != 0 {
		t.Fatalf("nothing is watched, yet chains=%v tokens=%v", chains, tokens)
	}
}

// The account handed to a payer comes from the WATCHED asset for their exact
// (chain, token) — never from a sibling token, and never invented.
func TestPooledAddressFor(t *testing.T) {
	assets := xrplWatched()

	if got := pooledAddressFor(assets, "xrpl", "rlusd"); got != poolAccount {
		t.Fatalf("pooledAddressFor(xrpl, rlusd) = %q, want %q", got, poolAccount)
	}
	// Case is normalised on both sides, so a request body spelling cannot
	// decide whether an address is found.
	if got := pooledAddressFor(assets, "XRPL", "RLUSD"); got != poolAccount {
		t.Fatalf("case variation lost the account: %q", got)
	}

	// THE fail-closed case. A token nothing watches has no account to give,
	// even though the CHAIN has one: handing over the pooled address for an
	// unwatched token invites a deposit no scan will ever look for.
	if got := pooledAddressFor(assets, "xrpl", "usdc"); got != "" {
		t.Fatalf("an unwatched token was handed the pooled account %q", got)
	}
	// And an unconfigured rail gives nothing at all, which is what makes the
	// mint path refuse rather than hand out an empty address.
	if got := pooledAddressFor(nil, "xrpl", "rlusd"); got != "" {
		t.Fatalf("an unwatched rail produced address %q", got)
	}
	// A per-payer chain never answers here; its address is minted per deposit.
	if got := pooledAddressFor(assets, "base", "usdc"); got != "" {
		t.Fatalf("a per-payer chain produced pooled address %q", got)
	}
}

// The mint path's guard, stated as the property the handler depends on: when
// no account is configured, there is nothing to hand out. The handler turns
// this empty string into a 503 rather than recording an intent with an empty
// DepositAddress — which the watcher would skip, silently, forever.
func TestPooledMint_RefusesWhenNoAccountIsConfigured(t *testing.T) {
	// Exactly the state of a deploy that set CRYPTO_DEPOSIT_RPC_XRPL and the
	// token but no CRYPTO_DEPOSIT_ADDRESS_XRPL. depositwatch refuses to build
	// such an asset at all, so the mint side sees no asset — and must refuse.
	if addr := pooledAddressFor(watchedAssets(), "xrpl", "rlusd"); addr != "" {
		t.Fatalf("an unconfigured process produced a custody account: %q", addr)
	}
}

// ── reuse ────────────────────────────────────────────────────────────────────

func orgCtx(org string) *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(context.Background(), org))
}

// seedPooledIntent writes an XRPL intent exactly as the pooled mint path does.
func seedPooledIntent(t *testing.T, org, payer, tag string, expires time.Duration) *cryptopaymentintent.CryptoPaymentIntent {
	t.Helper()
	in := cryptopaymentintent.New(orgCtx(org))
	in.Currency = "usd"
	in.Chain = cryptopaymentintent.XRPL
	in.Token = "rlusd"
	in.DepositAddress = poolAccount
	in.AddressTag = tag
	in.CustomerRef = payer
	in.Status = cryptopaymentintent.Pending
	in.ExpiresAt = time.Now().Add(expires)
	in.Defaults()
	if err := in.Create(); err != nil {
		t.Fatalf("seed intent: %v", err)
	}
	return in
}

// A payer refreshing the page gets THEIR TAG BACK, every time.
//
// Not a tag — the same one. A second tag for one payer is not merely wasteful:
// both intents stay watched forever (depositledger.Watched filters on the asset
// and never on status), so the finite tag space is consumed by page refreshes.
func TestOpenIntentFor_ReuseReturnsTheSameTag(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	const org, payer = "acme", "acme/alice"
	seedPooledIntent(t, org, payer, "7", 24*time.Hour)

	db := orgCtx(org)
	for i := 0; i < 5; i++ {
		got, ok := openIntentFor(db, payer, "xrpl", "rlusd")
		if !ok {
			t.Fatalf("refresh %d found no open intent to reuse — the payer would be issued a second tag", i)
		}
		if got.AddressTag != "7" {
			t.Fatalf("refresh %d returned tag %q, want %q", i, got.AddressTag, "7")
		}
		if got.DepositAddress != poolAccount {
			t.Fatalf("refresh %d returned address %q", i, got.DepositAddress)
		}
		// The response the payer actually receives must carry both halves. An
		// address without its tag is a payment credited to nobody.
		if resp := toCryptoDepositResponse(got); resp.AddressTag != "7" || resp.DepositAddress != poolAccount {
			t.Fatalf("the reused intent reaches the payer as %+v", resp)
		}
	}
}

// Tag "0" survives the whole round trip — store, reuse, and the JSON the payer
// reads. It is the first tag ever issued, so if anything treats it as absent
// the very first XRPL customer is the one who loses their deposit.
func TestOpenIntentFor_TagZeroIsCarried(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	const org, payer = "acme", "acme/zero"
	seedPooledIntent(t, org, payer, "0", 24*time.Hour)

	got, ok := openIntentFor(orgCtx(org), payer, "xrpl", "rlusd")
	if !ok {
		t.Fatal("an intent holding tag 0 was not reusable")
	}
	if got.AddressTag != "0" {
		t.Fatalf("tag 0 came back as %q — it was treated as absent somewhere", got.AddressTag)
	}
	resp := toCryptoDepositResponse(got)
	if resp.AddressTag != "0" {
		t.Fatalf("tag 0 reaches the payer as %q; they would send an untagged payment that credits nobody", resp.AddressTag)
	}
	// And it survives the WIRE. The field is omitempty, which is correct for
	// the per-payer chains that have no tag — but only because the tag is
	// rendered decimal, so tag zero is the string "0" and not "". An encoding
	// that made it numeric, or a zero-valued int, would be dropped here and the
	// payer would be shown an address with no tag.
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"addressTag":"0"`) {
		t.Fatalf("tag 0 is missing from the wire response: %s", b)
	}
}

// Reuse is scoped, and each scope has to hold or a tag would be shared.
func TestOpenIntentFor_DoesNotReuseWhatItMustNot(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	const org = "acme"
	seedPooledIntent(t, org, org+"/alice", "1", 24*time.Hour)

	db := orgCtx(org)
	t.Run("another payer's intent is not reusable", func(t *testing.T) {
		// The failure this prevents is not a wasted tag; it is TWO PAYERS
		// SHARING ONE, which credits one customer's deposit to the other.
		if in, ok := openIntentFor(db, org+"/bob", "xrpl", "rlusd"); ok {
			t.Fatalf("bob was handed alice's intent (tag %q)", in.AddressTag)
		}
	})
	t.Run("another asset is not reusable", func(t *testing.T) {
		if _, ok := openIntentFor(db, org+"/alice", "xrpl", "usdc"); ok {
			t.Fatal("an intent for a different token was reused")
		}
		if _, ok := openIntentFor(db, org+"/alice", "base", "rlusd"); ok {
			t.Fatal("an intent for a different chain was reused")
		}
	})
	t.Run("an expired intent is not reusable", func(t *testing.T) {
		seedPooledIntent(t, org, org+"/carol", "2", -time.Hour)
		if _, ok := openIntentFor(db, org+"/carol", "xrpl", "rlusd"); ok {
			t.Fatal("an expired intent was reused")
		}
	})
}

// A pooled intent that somehow lost its tag is a TRAP, not merely an
// incomplete record: the address on it is real and ours, so a payment to it
// arrives and is credited to nobody. It must not be handed to a payer.
//
// This handler cannot produce such a row — the tag is allocated and written in
// the same record as the address — but a restore or an older binary could, and
// the two tempting repairs are both worse than refusing: handing it over takes
// money we cannot route, and minting a replacement leaves the untagged row live
// and sprays the tag space on every refresh.
func TestDestinationIsComplete(t *testing.T) {
	pooled := func(tag string) *cryptopaymentintent.CryptoPaymentIntent {
		in := &cryptopaymentintent.CryptoPaymentIntent{}
		in.Chain, in.DepositAddress, in.AddressTag = cryptopaymentintent.XRPL, poolAccount, tag
		return in
	}
	if destinationIsComplete(pooled("")) {
		t.Fatal("a pooled intent with no tag was accepted as a usable destination — its deposits would be credited to nobody")
	}
	if !destinationIsComplete(pooled("0")) {
		t.Fatal("tag 0 was read as no tag")
	}
	if !destinationIsComplete(pooled("41")) {
		t.Fatal("a tagged pooled intent was rejected")
	}
	// Per-payer chains are untouched: the address IS the whole destination, so
	// a missing tag there is not a defect and must not start refusing.
	for _, chain := range []cryptopaymentintent.Chain{
		cryptopaymentintent.Ethereum, cryptopaymentintent.Base, cryptopaymentintent.Solana, cryptopaymentintent.TON,
	} {
		in := &cryptopaymentintent.CryptoPaymentIntent{}
		in.Chain, in.DepositAddress = chain, "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"
		if !destinationIsComplete(in) {
			t.Fatalf("a %s intent with no tag was refused — per-payer chains must be unaffected", chain)
		}
	}
}

// What is actually WRITTEN when a pooled deposit is minted — both halves of the
// destination, in one row.
//
// This is the statement a mutant once dropped without any test noticing, and
// the reason is worth keeping in view: it used to live inside
// CreateCryptoDeposit, which is gated shut, so nothing could reach it. The
// assertion is on the row read back from the store, not on the struct handed in.
func TestRecordIntent_PooledWritesBothHalves(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	const org, payer = "acme", "acme/alice"
	got, err := recordIntent(orgCtx(org), payer, "xrpl", "rlusd", 2500,
		processor.Wallet{Address: poolAccount}, "0")
	if err != nil {
		t.Fatalf("recordIntent: %v", err)
	}

	// Read it back the way the watcher will.
	reread := cryptopaymentintent.New(orgCtx(org))
	if err := reread.GetById(got.Id()); err != nil {
		t.Fatalf("re-read intent: %v", err)
	}
	if reread.DepositAddress != poolAccount {
		t.Fatalf("persisted address = %q, want %q", reread.DepositAddress, poolAccount)
	}
	if reread.AddressTag != "0" {
		t.Fatalf("persisted tag = %q, want %q — the address was recorded without the tag that says whose it is", reread.AddressTag, "0")
	}
	if reread.CustomerRef != payer {
		t.Fatalf("persisted payer = %q", reread.CustomerRef)
	}
	// The identity the watcher will key on must be the pooled one, not the
	// bare address — otherwise every payer collapses onto one identity.
	a := depositwatch.Asset{Chain: "xrpl", Token: "rlusd"}
	if id := a.Identity(reread.DepositAddress, reread.AddressTag); id != poolAccount+"#0" {
		t.Fatalf("the persisted row yields identity %q", id)
	}
}

// The write REFUSES an incomplete pooled destination, rather than recording a
// row that traps money. Enforced on the way in, because the address is real and
// ours the moment it is handed out: a payment reaches it whether or not we
// wrote down whose it is.
func TestRecordIntent_RefusesAnIncompleteDestination(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	if _, err := recordIntent(orgCtx("acme"), "acme/alice", "xrpl", "rlusd", 2500,
		processor.Wallet{Address: poolAccount}, ""); err == nil {
		t.Fatal("a pooled intent was recorded with an address and no tag — deposits to it would be credited to nobody")
	}
	if _, err := recordIntent(orgCtx("acme"), "acme/alice", "base", "usdc", 2500,
		processor.Wallet{}, ""); err == nil {
		t.Fatal("an intent was recorded with no address at all")
	}
}

// A per-payer mint is written exactly as it always was: address, wallet handle,
// and NO tag. The regression guard for "non-pooled chains are unaffected", at
// the point the row is created.
func TestRecordIntent_PerPayerIsUnchanged(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	const org, payer, addr = "acme", "acme/evm", "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"
	got, err := recordIntent(orgCtx(org), payer, "base", "usdc", 2500,
		processor.Wallet{Address: addr, ID: "wal_123"}, "")
	if err != nil {
		t.Fatalf("recordIntent: %v", err)
	}
	if got.DepositAddress != addr || got.WalletID != "wal_123" {
		t.Fatalf("per-payer intent recorded as address=%q wallet=%q", got.DepositAddress, got.WalletID)
	}
	if got.AddressTag != "" {
		t.Fatalf("a per-payer intent was tagged %q — it would name an identity no EVM deposit can match", got.AddressTag)
	}
	if got.Status != cryptopaymentintent.Pending {
		t.Fatalf("status = %q", got.Status)
	}
}

// A per-payer chain reuses exactly as it always did, and carries no tag. This
// is the regression guard for "non-pooled chains are completely unaffected".
func TestOpenIntentFor_APerPayerChainIsUnchangedAndUntagged(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	const org, payer = "acme", "acme/evm"
	in := cryptopaymentintent.New(orgCtx(org))
	in.Currency = "usd"
	in.Chain = cryptopaymentintent.Base
	in.Token = "usdc"
	in.DepositAddress = "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"
	in.WalletID = "wal_123"
	in.CustomerRef = payer
	in.Status = cryptopaymentintent.Pending
	in.ExpiresAt = time.Now().Add(24 * time.Hour)
	in.Defaults()
	if err := in.Create(); err != nil {
		t.Fatalf("seed intent: %v", err)
	}

	got, ok := openIntentFor(orgCtx(org), payer, "base", "usdc")
	if !ok {
		t.Fatal("a per-payer intent stopped being reusable")
	}
	if got.AddressTag != "" {
		t.Fatalf("a per-payer intent carries tag %q — it names an identity no EVM deposit can ever match", got.AddressTag)
	}
	resp := toCryptoDepositResponse(got)
	if resp.AddressTag != "" {
		t.Fatalf("a per-payer response carries a tag: %+v", resp)
	}
	if resp.DepositAddress != in.DepositAddress {
		t.Fatalf("the per-payer address changed: %q", resp.DepositAddress)
	}
}
