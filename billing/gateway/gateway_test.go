// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"context"
	"encoding/hex"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/luxfi/crypto"

	"github.com/hanzoai/commerce/billing/husdindex"
	"github.com/hanzoai/commerce/models/remittance"
	"github.com/hanzoai/commerce/payment/x402"
)

// The four bytes that pick a function, and the topic that names an event, are
// the keccak of their signatures, or the rail calls something else.
func TestSelectorsAreTheTokensOwn(t *testing.T) {
	for sig, want := range map[string]string{
		"transferWithAuthorization(address,address,uint256,uint256,uint256,bytes32,uint8,bytes32,bytes32)": transferSelector,
		"authorizationState(address,bytes32)": stateSelector,
		"DOMAIN_SEPARATOR()":                  separatorSelector,
		"decimals()":                          decimalsSelector,
		"balanceOf(address)":                  balanceSelector,
	} {
		if got := hex.EncodeToString(crypto.Keccak256([]byte(sig))[:4]); got != want {
			t.Errorf("%s = %s, want %s", sig, got, want)
		}
	}
	if got := x402.Hex(crypto.Keccak256([]byte("AuthorizationUsed(address,bytes32)"))); got != usedTopic {
		t.Errorf("AuthorizationUsed topic = %s, want %s", got, usedTopic)
	}
	if got := x402.Hex(crypto.Keccak256([]byte("Transfer(address,address,uint256)"))); got != husdindex.TransferTopic0 {
		t.Errorf("Transfer topic = %s", got)
	}
}

// The table's domains are the tokens' own, as their DOMAIN_SEPARATOR() states
// it on chain (read 2026-10-30).
func TestTheTableNamesTheTokensDomains(t *testing.T) {
	for c, want := range map[*Chain]string{
		&mainnet[0]: "02fa7265e7c5d81118673727957699e4d68f74cd74b7db77da710fe8a2c7834f",
		&testnet[0]: "71f17a3b2ff373b803d70a5a07c046c1a2bc8e89c09ef722fcb047abe94c9818",
	} {
		sep, err := c.Domain().Separator()
		if err != nil || hex.EncodeToString(sep[:]) != want {
			t.Errorf("%s: separator %x (%v), the token says %s", c.ID, sep, err, want)
		}
	}
}

// The whole path: terms, a signature, the relayer's transaction, and a credit
// only once the transaction is Confirmations deep — exactly once, however
// often the payment is presented or the reconciler runs.
func TestAPaymentIsRelayedAndCreditedOnceConfirmed(t *testing.T) {
	r := newRig(t, false)
	terms := r.terms()
	if len(terms.Accepts) != 1 || terms.Accepts[0].Domain != r.chain.Domain() || terms.PayTo != treasury || terms.Scheme != "exact" {
		t.Fatalf("terms: %+v", terms)
	}
	sub := r.sign(terms, 2500, nil)

	v, err := r.submit(sub)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if r.node.relayed() != 1 || v.TxHash == "" {
		t.Fatalf("relayed %d, view %+v", r.node.relayed(), v)
	}
	if v.Credited || r.credited() != 0 {
		t.Fatalf("credited before any confirmation: %+v, ledger %d", v, r.credited())
	}
	if r.node.usdcBalance(treasury) != 2500 || r.node.usdcBalance(r.from) != 100_000-2500 {
		t.Fatalf("the token moved %d to the treasury", r.node.usdcBalance(treasury))
	}
	rec := r.stored(sub)
	if len(rec.Sends) != 1 || rec.Relayer != r.relayer {
		t.Fatalf("the relayer's send is not on the record: %+v", rec)
	}

	// Mined, not deep enough: confirming, nothing credited.
	r.step()
	if rec = r.stored(sub); rec.Status != remittance.Confirming || rec.Credited || r.credited() != 0 {
		t.Fatalf("one block deep: %s credited=%v ledger=%d", rec.Status, rec.Credited, r.credited())
	}
	r.node.mine(uint64(r.chain.Confirmations) - 2)
	r.step()
	if rec = r.stored(sub); rec.Credited || r.credited() != 0 {
		t.Fatalf("%d of %d blocks: credited", rec.Confirmations, r.chain.Confirmations)
	}

	// Deep enough: credited, once.
	r.node.mine(1)
	r.step()
	if rec = r.stored(sub); rec.Status != remittance.Succeeded || !rec.Landed || r.credited() != 2500 {
		t.Fatalf("deep enough: %s landed=%v ledger=%d", rec.Status, rec.Landed, r.credited())
	}
	if got := r.receipts(); got.PrepaidBalance != 2500 {
		t.Fatalf("receipt: %+v", got)
	}
	v, err = r.submit(sub)
	if err != nil || v.Status != remittance.Succeeded || !v.Credited {
		t.Fatalf("presented again: %+v %v", v, err)
	}
	r.pass()
	r.pass()
	if r.credited() != 2500 || r.node.relayed() != 1 || len(r.book.credits) != 1 {
		t.Fatalf("after replays: ledger %d, relayed %d, credits %d", r.credited(), r.node.relayed(), len(r.book.credits))
	}
	if got, err := r.s.Get(context.Background(), r.org, r.payer, v.ID); err != nil || got.Status != remittance.Succeeded || got.TxHash != rec.TxHash {
		t.Fatalf("read back: %+v %v", got, err)
	}
}

// Every check the rail can make without the chain refuses with its reason,
// records nothing and sends nothing.
func TestAPaymentTheRailCannotTakeIsRefusedUpFront(t *testing.T) {
	r := newRig(t, false)
	other := mustKey(t)
	for name, tc := range map[string]struct {
		edit   func(*Submission)
		reason string
	}{
		"another payee":         {func(s *Submission) { s.Authorization.To = "0x" + strings.Repeat("ab", 20) }, ReasonRecipient},
		"part of a cent":        {func(s *Submission) { s.Authorization.Value = "1000001" }, ReasonCents},
		"below the minimum":     {func(s *Submission) { s.Authorization.Value = "990000" }, ReasonLow},
		"above the maximum":     {func(s *Submission) { s.Authorization.Value = "1000010000" }, ReasonHigh},
		"a nonce not issued":    {func(s *Submission) { s.Authorization.Nonce = random32() }, ReasonUnbound},
		"another caller's salt": {func(s *Submission) { s.Salt = random32() }, ReasonUnbound},
		"unsupported network":   {func(s *Submission) { s.Network = "eip155:1" }, ReasonNetwork},
		"not yet valid": {func(s *Submission) {
			s.Authorization.ValidAfter = strconv.FormatInt(r.now.Add(time.Minute).Unix(), 10)
		}, ReasonEarly},
		"too old": {func(s *Submission) {
			s.Authorization.ValidAfter = strconv.FormatInt(r.now.Add(-25*time.Hour).Unix(), 10)
		}, ReasonStale},
		"too short": {func(s *Submission) {
			s.Authorization.ValidBefore = strconv.FormatInt(r.now.Add(time.Minute).Unix(), 10)
		}, ReasonShort},
		"too long": {func(s *Submission) {
			s.Authorization.ValidBefore = strconv.FormatInt(r.now.Add(25*time.Hour).Unix(), 10)
		}, ReasonLong},
		"expired": {func(s *Submission) {
			s.Authorization.ValidBefore = strconv.FormatInt(r.now.Add(-time.Second).Unix(), 10)
		}, ReasonExpired},
	} {
		t.Run(name, func(t *testing.T) {
			sub := r.pay(500, nil)
			tc.edit(&sub)
			// Re-sign what was edited, so the signature is never the reason.
			if sub.Network == r.chain.ID {
				sub.Signature = signWith(t, r.key, r.chain.Domain(), sub.Authorization)
			}
			_, err := r.submit(sub)
			if ReasonOf(err) != tc.reason || StatusOf(err) != 400 {
				t.Fatalf("got %d %q (%v), want 400 %q", StatusOf(err), ReasonOf(err), err, tc.reason)
			}
			if sub.Network == r.chain.ID && r.exists(sub) {
				t.Fatal("a refused payment was recorded")
			}
		})
	}
	for name, sig := range map[string]func(Submission) string{
		"signed by another key": func(s Submission) string { return signWith(t, other, r.chain.Domain(), s.Authorization) },
		"signed for another chain": func(s Submission) string {
			d := r.chain.Domain()
			d.ChainID = 8453
			return signWith(t, r.key, d, s.Authorization)
		},
		"signed for another domain": func(s Submission) string {
			d := r.chain.Domain()
			d.Name = "USD Coin"
			return signWith(t, r.key, d, s.Authorization)
		},
		"high s": func(s Submission) string {
			b, _ := hex.DecodeString(strings.TrimPrefix(s.Signature, "0x"))
			n := crypto.S256().Params().N
			hi := new(big.Int).Sub(n, new(big.Int).SetBytes(b[32:64]))
			hi.FillBytes(b[32:64])
			b[64] ^= 1
			return x402.Hex(b)
		},
	} {
		t.Run(name, func(t *testing.T) {
			sub := r.pay(500, nil)
			sub.Signature = sig(sub)
			if _, err := r.submit(sub); ReasonOf(err) != ReasonSignature {
				t.Fatalf("got %q (%v), want %q", ReasonOf(err), err, ReasonSignature)
			}
			if r.exists(sub) {
				t.Fatal("a refused payment was recorded")
			}
		})
	}
	if r.node.relayed() != 0 {
		t.Fatalf("the relayer sent %d transactions for refused payments", r.node.relayed())
	}
}

// A wallet without the USDC is told so, and nothing is sent; funded, the same
// signed payment goes through.
func TestAnUnfundedWalletIsRefusedThenPaysWhenFunded(t *testing.T) {
	r := newRig(t, false)
	sub := r.pay(200_00, nil)
	r.node.set(func(n *node) { n.usdcOf[r.from] = big.NewInt(100 * 10_000) })
	_, err := r.submit(sub)
	if ReasonOf(err) != ReasonBalance || StatusOf(err) != 400 {
		t.Fatalf("got %d %q (%v)", StatusOf(err), ReasonOf(err), err)
	}
	if rec := r.stored(sub); rec.Status != remittance.Refused || len(rec.Sends) != 0 || r.node.relayed() != 0 {
		t.Fatalf("refused: %+v, relayed %d", rec, r.node.relayed())
	}
	r.node.fund(r.from, 200_00)
	if _, err := r.submit(sub); err != nil {
		t.Fatalf("presented again, funded: %v", err)
	}
	r.node.mine(uint64(r.chain.Confirmations))
	r.step()
	if r.credited() != 200_00 || r.node.relayed() != 1 {
		t.Fatalf("ledger %d, relayed %d", r.credited(), r.node.relayed())
	}
}

// The authorization fixes the payee and the amount, so whoever submits it pays
// the treasury: a use the rail did not send is found on chain and credited,
// once, and the relayer's own transaction is never sent.
func TestAUseTheRailDidNotSendIsCredited(t *testing.T) {
	for _, when := range []string{"before it is presented", "after it is refused"} {
		t.Run(when, func(t *testing.T) {
			r := newRig(t, false)
			sub := r.pay(700, nil)
			if when == "after it is refused" {
				r.node.set(func(n *node) { n.usdcOf[r.from] = new(big.Int) })
				if _, err := r.submit(sub); ReasonOf(err) != ReasonBalance {
					t.Fatalf("refusal: %v", err)
				}
				r.node.fund(r.from, 700)
			}
			r.node.use(sub)
			r.node.mine(uint64(r.chain.Confirmations))
			if when == "before it is presented" {
				if _, err := r.submit(sub); err != nil {
					t.Fatalf("submit: %v", err)
				}
			} else {
				r.pass()
			}
			r.step()
			if rec := r.stored(sub); rec.Status != remittance.Succeeded || r.credited() != 700 || r.node.relayed() != 0 {
				t.Fatalf("%s: ledger %d, relayed %d", rec.Status, r.credited(), r.node.relayed())
			}
		})
	}
}

// A transaction the node dropped is sent again — and the record names both.
func TestADroppedTransactionIsSentAgain(t *testing.T) {
	r := newRig(t, false)
	r.node.set(func(n *node) { n.drop = true })
	sub := r.pay(300, nil)
	if _, err := r.submit(sub); err != nil {
		t.Fatal(err)
	}
	r.s.Reconcile(context.Background()) // not due, and inside the resend wait
	if r.node.relayed() != 1 {
		t.Fatalf("sent again inside the wait: %d", r.node.relayed())
	}
	r.node.set(func(n *node) { n.drop = false })
	r.step()
	r.node.mine(uint64(r.chain.Confirmations))
	r.step()
	rec := r.stored(sub)
	if len(rec.Sends) != 2 || rec.Status != remittance.Succeeded || r.credited() != 300 {
		t.Fatalf("sends %d, %s, ledger %d", len(rec.Sends), rec.Status, r.credited())
	}
}

// A transaction still in the pool is waited for, not sent again.
func TestATransactionInThePoolIsWaitedFor(t *testing.T) {
	r := newRig(t, false)
	r.node.set(func(n *node) { n.hold = true })
	sub := r.pay(300, nil)
	if _, err := r.submit(sub); err != nil {
		t.Fatal(err)
	}
	r.step()
	r.step()
	if r.node.relayed() != 1 {
		t.Fatalf("a pooled transaction was sent again: %d", r.node.relayed())
	}
	r.node.release()
	r.node.mine(uint64(r.chain.Confirmations))
	r.step()
	r.step()
	if r.credited() != 300 {
		t.Fatalf("ledger %d", r.credited())
	}
}

// A use the chain takes back before it is deep enough is never credited; the
// payment is followed again and credited once its new use is deep.
func TestAReorganisedUseIsNotCredited(t *testing.T) {
	r := newRig(t, false)
	sub := r.pay(900, nil)
	v, err := r.submit(sub)
	if err != nil {
		t.Fatal(err)
	}
	r.step()
	r.node.mine(uint64(r.chain.Confirmations) - 2)
	r.node.reorg(v.TxHash)
	r.node.mine(5)
	r.step()
	if r.credited() != 0 {
		t.Fatalf("credited a use the chain took back: %d", r.credited())
	}
	r.step()
	r.node.mine(uint64(r.chain.Confirmations))
	r.step()
	r.step()
	if rec := r.stored(sub); rec.Status != remittance.Succeeded || r.credited() != 900 || len(rec.Sends) != 2 {
		t.Fatalf("%s, ledger %d, sends %d", rec.Status, r.credited(), len(rec.Sends))
	}
}

// An authorization never used fails once it has expired, uncredited.
func TestAnUnusedAuthorizationFailsOnceExpired(t *testing.T) {
	r := newRig(t, false)
	r.node.set(func(n *node) { n.drop = true })
	sub := r.pay(400, func(a *x402.Authorization) {
		a.ValidBefore = strconv.FormatInt(r.now.Add(10*time.Minute).Unix(), 10)
	})
	if _, err := r.submit(sub); err != nil {
		t.Fatal(err)
	}
	r.node.set(func(n *node) { n.drop, n.down = false, true })
	r.pass()
	r.node.set(func(n *node) { n.down = false })
	r.pass()
	if rec := r.stored(sub); rec.Status != remittance.Failed || rec.Reason != ReasonExpired || r.credited() != 0 {
		t.Fatalf("%s %q, ledger %d", rec.Status, rec.Reason, r.credited())
	}
}

// A payer who signs a second authorization under the same nonce and spends it
// elsewhere leaves the first unable to land: it fails, uncredited.
func TestANonceSpentOnAnotherAuthorizationFails(t *testing.T) {
	r := newRig(t, false)
	terms := r.terms()
	ours := r.sign(terms, 500, nil)
	theirs := r.sign(terms, 600, nil) // same nonce, another value
	r.node.set(func(n *node) { n.hold = true })
	if _, err := r.submit(ours); err != nil {
		t.Fatal(err)
	}
	r.node.set(func(n *node) { n.pool = map[string]*authorization{}; n.hold = false })
	r.node.use(theirs)
	r.step()
	r.step()
	if rec := r.stored(ours); rec.Status != remittance.Failed || rec.Reason != ReasonConflict || r.credited() != 0 {
		t.Fatalf("%s %q, ledger %d", rec.Status, rec.Reason, r.credited())
	}
}

// A relayer that cannot pay the gas is a 503 and nothing moves; funded, the
// reconciler sends the recorded payment.
func TestAnUnfundedRelayerSendsNothingUntilFunded(t *testing.T) {
	r := newRig(t, false)
	r.node.gas(r.relayer, new(big.Int))
	sub := r.pay(500, nil)
	_, err := r.submit(sub)
	if ReasonOf(err) != ReasonRelayer || StatusOf(err) != 503 {
		t.Fatalf("got %d %q", StatusOf(err), ReasonOf(err))
	}
	if rec := r.stored(sub); rec.Status != remittance.Pending || len(rec.Sends) != 0 || r.node.relayed() != 0 {
		t.Fatalf("%+v", rec)
	}
	r.node.gas(r.relayer, big.NewInt(1e18))
	r.pass()
	r.node.mine(uint64(r.chain.Confirmations))
	r.step()
	if r.credited() != 500 || r.node.relayed() != 1 {
		t.Fatalf("ledger %d, relayed %d", r.credited(), r.node.relayed())
	}
}

// A credit the ledger did not take is owed, and landed by the reconciler.
func TestACreditTheLedgerRefusedIsLandedLater(t *testing.T) {
	r := newRig(t, false)
	sub := r.pay(800, nil)
	if _, err := r.submit(sub); err != nil {
		t.Fatal(err)
	}
	r.book.set(func(b *book) { b.broken = true })
	r.node.mine(uint64(r.chain.Confirmations))
	r.step()
	if rec := r.stored(sub); !rec.Credited || rec.Landed || rec.Status != remittance.Confirming {
		t.Fatalf("owed: %+v", rec)
	}
	r.book.set(func(b *book) { b.broken = false })
	r.pass()
	if rec := r.stored(sub); rec.Status != remittance.Succeeded || r.credited() != 800 {
		t.Fatalf("%s, ledger %d", rec.Status, r.credited())
	}
}

// Sixteen presentations at once are one payment: one record, one relayer
// transaction, one credit.
func TestConcurrentPresentationsAreOnePayment(t *testing.T) {
	r := newRig(t, false)
	sub := r.pay(1000, nil)
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = r.submit(sub)
		}()
	}
	wg.Wait()
	r.node.mine(uint64(r.chain.Confirmations))
	r.step()
	r.pass()
	if r.node.relayed() != 1 || r.credited() != 1000 || len(r.stored(sub).Sends) != 1 {
		t.Fatalf("relayed %d, ledger %d, sends %d", r.node.relayed(), r.credited(), len(r.stored(sub).Sends))
	}
}

// The same nonce presented as a different payment is a conflict, and one
// presented by another caller is not theirs to present.
func TestANonceNamesOnePaymentForOneCaller(t *testing.T) {
	r := newRig(t, false)
	terms := r.terms()
	first := r.sign(terms, 500, nil)
	if _, err := r.submit(first); err != nil {
		t.Fatal(err)
	}
	again := r.sign(terms, 500, func(a *x402.Authorization) {
		a.ValidBefore = strconv.FormatInt(r.now.Add(2*time.Hour).Unix(), 10)
	})
	if _, err := r.submit(again); ReasonOf(err) != ReasonConflict {
		t.Fatalf("a second payment under one nonce: %v", err)
	}
	stranger := *r.org
	stranger.Name = r.org.Name + "x"
	if _, err := r.s.Submit(context.Background(), &stranger, stranger.Name, first); ReasonOf(err) != ReasonUnbound {
		t.Fatalf("another caller presenting it: %v", err)
	}
	v, _ := r.s.Get(context.Background(), r.org, r.payer, ID(first.Network, first.Authorization.From, first.Authorization.Nonce))
	if _, err := r.s.Get(context.Background(), &stranger, stranger.Name, v.ID); ReasonOf(err) != ReasonMissing {
		t.Fatalf("another caller reading it: %v", err)
	}
}

// Testnet money funds test books only, and mainnet money live books only.
func TestBooksFollowTheNetwork(t *testing.T) {
	test := newRig(t, false)
	live := *test.org
	live.Live = true
	if _, err := test.s.Terms(context.Background(), &live, live.Name); ReasonOf(err) != ReasonBooks {
		t.Fatalf("testnet rail, live books: %v", err)
	}
	main := newRig(t, true)
	sandbox := *main.org
	sandbox.Live = false
	if _, err := main.s.Terms(context.Background(), &sandbox, sandbox.Name); ReasonOf(err) != ReasonBooks {
		t.Fatalf("mainnet rail, test books: %v", err)
	}
}

// A node that is not the chain the table names, or a token whose domain is not
// the one the rail hands payers, is never offered.
func TestANetworkIsOfferedOnlyWhenTheChainAgrees(t *testing.T) {
	r := newRig(t, false)
	r.node.set(func(n *node) { n.separator = x402.Hex(make([]byte, 32)) })
	if _, err := r.s.Terms(context.Background(), r.org, r.payer); ReasonOf(err) != ReasonUnavailable {
		t.Fatalf("a token with another domain: %v", err)
	}
	if got := r.s.Networks(); len(got) != 0 {
		t.Fatalf("menu offers %v", got)
	}
	r.node.set(func(n *node) { n.separator = "" })
	r.now = r.now.Add(failAge + time.Second)
	if got := r.terms(); len(got.Accepts) != 1 {
		t.Fatalf("terms after the token agrees: %+v", got)
	}
	if got := r.s.Networks(); len(got) != 1 || got[0] != r.chain.ID {
		t.Fatalf("menu: %v", got)
	}

	other := newRig(t, false)
	other.node.set(func(n *node) { n.chainID = 1 })
	if _, err := other.s.Terms(context.Background(), other.org, other.payer); ReasonOf(err) != ReasonUnavailable {
		t.Fatalf("a node on another chain: %v", err)
	}
}

func TestFromEnv(t *testing.T) {
	key := strings.Repeat("11", 32)
	base := []string{EnvTreasury + "=" + treasury, EnvNetwork + "=mainnet", EnvRelayer + "=" + key, "CRYPTO_DEPOSIT_RPC_BASE=https://mainnet.base.org"}
	cfg, err := FromEnv(base)
	if err != nil || !cfg.Armed() || cfg.Testnet || len(cfg.Chains) != 1 || cfg.Chains[0].RPC != "https://mainnet.base.org" {
		t.Fatalf("armed: %+v %v", cfg, err)
	}
	if cfg, err := FromEnv(nil); err != nil || cfg.Armed() {
		t.Fatalf("unset is inert: %+v %v", cfg, err)
	}
	test, err := FromEnv([]string{EnvTreasury + "=" + treasury, EnvNetwork + "=testnet", EnvRelayer + "=0x" + key, "CRYPTO_DEPOSIT_RPC_BASE_SEPOLIA=https://sepolia.base.org"})
	if err != nil || !test.Testnet || test.Chains[0].ChainID != 84532 {
		t.Fatalf("testnet: %+v %v", test, err)
	}
	relayer, _ := crypto.HexToECDSA(key)
	same := x402.Hex(crypto.PubkeyToAddress(relayer.PublicKey).Bytes())
	for name, env := range map[string][]string{
		"no network":       {EnvTreasury + "=" + treasury, EnvRelayer + "=" + key, "CRYPTO_DEPOSIT_RPC_BASE=x"},
		"no relayer":       {EnvTreasury + "=" + treasury, EnvNetwork + "=mainnet", "CRYPTO_DEPOSIT_RPC_BASE=x"},
		"bad relayer":      {EnvTreasury + "=" + treasury, EnvNetwork + "=mainnet", EnvRelayer + "=zz" + key[2:], "CRYPTO_DEPOSIT_RPC_BASE=x"},
		"no rpc":           {EnvTreasury + "=" + treasury, EnvNetwork + "=mainnet", EnvRelayer + "=" + key},
		"testnet, no rpc":  {EnvTreasury + "=" + treasury, EnvNetwork + "=testnet", EnvRelayer + "=" + key, "CRYPTO_DEPOSIT_RPC_BASE=x"},
		"bad treasury":     {EnvTreasury + "=0x1234", EnvNetwork + "=mainnet", EnvRelayer + "=" + key, "CRYPTO_DEPOSIT_RPC_BASE=x"},
		"zero treasury":    {EnvTreasury + "=0x" + strings.Repeat("0", 40), EnvNetwork + "=mainnet", EnvRelayer + "=" + key, "CRYPTO_DEPOSIT_RPC_BASE=x"},
		"relayer treasury": {EnvTreasury + "=" + same, EnvNetwork + "=mainnet", EnvRelayer + "=" + key, "CRYPTO_DEPOSIT_RPC_BASE=x"},
	} {
		_, err := FromEnv(env)
		if err == nil {
			t.Errorf("%s: armed", name)
		} else if strings.Contains(err.Error(), key[4:]) {
			t.Errorf("%s: the error carries the key", name)
		}
	}
}

// The calldata is transferWithAuthorization's ABI, word for word: what the
// token decodes is what the payer signed, and v is 27 or 28.
func TestTheCallCarriesThePaymentAsSigned(t *testing.T) {
	r := newRig(t, false)
	sub := r.pay(1234, nil)
	if _, err := r.submit(sub); err != nil {
		t.Fatal(err)
	}
	data, err := transferCall(r.stored(sub))
	if err != nil {
		t.Fatal(err)
	}
	a, err := decodeCall(data)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := sub.Authorization.Parse()
	if a.from != p.From || a.to != p.To || a.value.Cmp(p.Value) != 0 || a.nonce != p.Nonce ||
		a.validAfter.Uint64() != p.ValidAfter || a.validBefore.Uint64() != p.ValidBefore || (a.v != 27 && a.v != 28) {
		t.Fatalf("decoded %+v from %+v", a, p)
	}
}

// A payment presented again, or looked at by the reconciler, while its
// relayer transaction has not had time to show up is not sent again: a node
// that has not propagated a transaction yet does not hold it either.
func TestARelayedPaymentIsNotSentAgainInsideTheWait(t *testing.T) {
	r := newRig(t, false)
	r.node.set(func(n *node) { n.drop = true })
	sub := r.pay(300, nil)
	if _, err := r.submit(sub); err != nil {
		t.Fatal(err)
	}
	if _, err := r.submit(sub); err != nil {
		t.Fatal(err)
	}
	r.now = r.now.Add(time.Minute + time.Second) // due, and inside the wait
	r.s.Reconcile(context.Background())
	if r.node.relayed() != 1 {
		t.Fatalf("sent %d times inside the wait", r.node.relayed())
	}
}

// A landing whose flag was lost after the ledger took the credit is landed
// again as a keyed replay: the ledger holds one credit.
func TestALandingIsAKeyedReplay(t *testing.T) {
	r := newRig(t, false)
	sub := r.pay(600, nil)
	if _, err := r.submit(sub); err != nil {
		t.Fatal(err)
	}
	r.node.mine(uint64(r.chain.Confirmations))
	r.step()
	id := r.stored(sub).Id()
	if _, err := update(id, func(x *remittance.Remittance) bool {
		// As a crash between the ledger's answer and the flag leaves it: the
		// landing's own swap never ran.
		x.Landed, x.Status, x.Due = false, remittance.Confirming, stamp(r.now)
		return true
	}); err != nil {
		t.Fatal(err)
	}
	r.pass()
	if rec := r.stored(sub); rec.Status != remittance.Succeeded || r.credited() != 600 || r.book.calls != 2 {
		t.Fatalf("%s, ledger %d, ledger calls %d", rec.Status, r.credited(), r.book.calls)
	}
	if got := r.receipts(); got.PrepaidBalance != 600 {
		t.Fatalf("receipts: %+v", got)
	}
}

// The operator's view reads the chain: the treasury's USDC, the relayer's gas,
// and what is still open — a refused payment is open but owes nothing.
func TestTheTreasuryViewReadsTheChain(t *testing.T) {
	r := newRig(t, false)
	paid := r.pay(1500, nil)
	if _, err := r.submit(paid); err != nil {
		t.Fatal(err)
	}
	r.node.set(func(n *node) { n.usdcOf[r.from] = new(big.Int) })
	if _, err := r.submit(r.pay(200, nil)); ReasonOf(err) != ReasonBalance {
		t.Fatalf("refusal: %v", err)
	}
	v, err := r.s.Treasury(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v.Network != "testnet" || v.Treasury != treasury || v.Relayer != r.relayer || len(v.Chains) != 1 {
		t.Fatalf("%+v", v)
	}
	if c := v.Chains[0]; c.USDC != "15000000" || c.RelayerWei == "" || c.Error != "" {
		t.Fatalf("holding: %+v", c)
	}
	if v.Open.Pending != 1 || v.Open.Refused != 1 || v.Open.Cents != 1500 {
		t.Fatalf("open: %+v", v.Open)
	}
}
