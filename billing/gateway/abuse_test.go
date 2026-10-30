// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// The red team's proofs against the rail, kept as its regression tests: each
// once paid out more than arrived, burned the relayer's gas, or never ended.

package gateway

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/luxfi/crypto"

	"github.com/hanzoai/commerce/billing/husdindex"
	"github.com/hanzoai/commerce/models/remittance"
	"github.com/hanzoai/commerce/payment/eip3009"
)

func authOf(sub Submission) authorization {
	p, _ := sub.Authorization.Parse()
	sig, _ := hex.DecodeString(strings.TrimPrefix(sub.Signature, "0x"))
	a := authorization{from: p.From, to: p.To, value: p.Value,
		validAfter: new(big.Int).SetUint64(p.ValidAfter), validBefore: new(big.Int).SetUint64(p.ValidBefore), nonce: p.Nonce}
	copy(a.r[:], sig[:32])
	copy(a.s[:], sig[32:64])
	a.v = sig[64]
	if a.v < 27 {
		a.v += 27
	}
	return a
}

// bundle mines ONE transaction that calls transferWithAuthorization once per
// submission, in order — what any multicall contract does. Logs are exactly
// FiatTokenV2's: AuthorizationUsed then Transfer, per call.
func (n *node) bundle(subs ...Submission) string {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.head++
	hash := eip3009.Hex(crypto.Keccak256([]byte(fmt.Sprintf("bundle-%d", len(n.mined)))))
	m := &mined{block: n.head, success: true, undo: func() {}}
	word := func(b []byte) string { return eip3009.Hex(pad(b)) }
	for _, sub := range subs {
		a := authOf(sub)
		if why := n.check(a); why != "" {
			panic("bundle call reverts: " + why)
		}
		from, to := eip3009.Hex(a.from[:]), eip3009.Hex(a.to[:])
		n.used[from+"|"+eip3009.Hex(a.nonce[:])] = true
		n.usdcOf[from].Sub(n.usdcOf[from], a.value)
		if n.usdcOf[to] == nil {
			n.usdcOf[to] = new(big.Int)
		}
		n.usdcOf[to].Add(n.usdcOf[to], a.value)
		m.logs = append(m.logs,
			map[string]any{"address": n.usdc, "topics": []string{usedTopic, word(a.from[:]), eip3009.Hex(a.nonce[:])}, "data": "0x"},
			map[string]any{"address": n.usdc, "topics": []string{husdindex.TransferTopic0, word(a.from[:]), word(a.to[:])}, "data": eip3009.Hex(pad(a.value.Bytes()))},
		)
	}
	n.mined[hash] = m
	return hash
}

// One $25 transfer into the treasury credits k * $25: every other issued nonce
// is burned in the same transaction by a ZERO-value authorization to an
// address the attacker owns, and spent() accepts the lone Transfer(payer ->
// treasury, 25.00) as proof for each of them.
func TestOneTransferCreditsOnePaymentHoweverManyShareItsTransaction(t *testing.T) {
	const k = 10
	r := newRig(t, false)
	r.node.set(func(n *node) { n.usdcOf[r.from] = big.NewInt(2500 * 10_000) }) // the wallet holds $25 and nothing more
	sink := eip3009.Hex(crypto.PubkeyToAddress(mustKey(t).PublicKey).Bytes())

	var rail []Submission  // what is presented to the rail: k payments of $25 to the treasury
	var chain []Submission // what is mined: one real $25, k-1 zero-value burns of the other nonces
	for i := 0; i < k; i++ {
		terms := r.terms()
		rail = append(rail, r.sign(terms, 2500, nil))
		if i == 0 {
			chain = append(chain, rail[0])
			continue
		}
		chain = append(chain, r.sign(terms, 0, func(a *eip3009.Authorization) { a.To, a.Value = sink, "0" }))
	}
	r.node.bundle(chain...)

	for _, s := range rail {
		if _, err := r.submit(s); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	r.node.mine(uint64(r.chain.Confirmations))
	r.step()
	r.pass()

	n := 0
	for _, s := range rail {
		if r.stored(s).Status == remittance.Succeeded {
			n++
		}
	}
	t.Logf("treasury received %d cents; payer credited %d cents; %d of %d remittances succeeded; relayer sends %d",
		r.node.usdcBalance(treasury), r.credited(), n, k, r.node.relayed())
	if r.credited() > int64(r.node.usdcBalance(treasury)) {
		t.Errorf("EXPLOIT: credited %d cents for %d cents received", r.credited(), r.node.usdcBalance(treasury))
	}
}

// The same with no front-run at all: present every payment while the wallet is
// empty (refused, then watched), then mine the bundle.
func TestRefusedPaymentsBundledLaterCreditOnlyWhatArrived(t *testing.T) {
	const k = 4
	r := newRig(t, false)
	r.node.set(func(n *node) { n.usdcOf[r.from] = new(big.Int) })
	sink := eip3009.Hex(crypto.PubkeyToAddress(mustKey(t).PublicKey).Bytes())
	var rail, chain []Submission
	for i := 0; i < k; i++ {
		terms := r.terms()
		rail = append(rail, r.sign(terms, 100_000, nil)) // $1,000 each, the maximum
		if _, err := r.submit(rail[i]); err == nil || ReasonOf(err) != ReasonBalance {
			t.Fatalf("want insufficient_balance refusal, got %v", err)
		}
		if i == 0 {
			chain = append(chain, rail[0])
		} else {
			chain = append(chain, r.sign(terms, 0, func(a *eip3009.Authorization) { a.To, a.Value = sink, "0" }))
		}
	}
	r.node.fund(r.from, 100_000)
	r.node.bundle(chain...)
	for i := 0; i < 4; i++ {
		r.node.mine(uint64(r.chain.Confirmations))
		r.pass()
	}
	t.Logf("treasury received %d cents; payer credited %d cents; relayer sends %d", r.node.usdcBalance(treasury), r.credited(), r.node.relayed())
	if r.credited() > int64(r.node.usdcBalance(treasury)) {
		t.Errorf("EXPLOIT: credited %d cents for %d cents received", r.credited(), r.node.usdcBalance(treasury))
	}
}

// k payments presented at once from a wallet that can fund only one: once
// every simulation passed against "latest", so the relayer sent k
// transactions and k-1 reverted on chain, at Hanzo's gas. Now the wallet must
// fund every payment pending from it, and has one in flight at a time: the
// first is sent, the rest are refused or wait, and nothing reverts.
func TestAnOverdrawnWalletCostsTheRelayerNothing(t *testing.T) {
	const k = 8
	r := newRig(t, false)
	r.node.set(func(n *node) { n.usdcOf[r.from] = big.NewInt(100 * 10_000); n.hold = true }) // $1 in the wallet; txs sit in the pool like a real block interval
	var subs []Submission
	for i := 0; i < k; i++ {
		subs = append(subs, r.pay(100, nil))
	}
	for _, s := range subs {
		if _, err := r.submit(s); err != nil && ReasonOf(err) != ReasonBalance && ReasonOf(err) != ReasonOpen {
			t.Fatalf("submit: %v", err)
		}
	}
	r.node.release()
	for range 3 {
		r.step()
	}
	reverted := 0
	r.node.set(func(n *node) {
		for _, m := range n.mined {
			if !m.success {
				reverted++
			}
		}
	})
	t.Logf("relayer sent %d transactions; %d reverted on chain at the relayer's expense; credited %d", r.node.relayed(), reverted, r.credited())
	if reverted > 0 || r.node.relayed() != 1 {
		t.Errorf("GRIEF: %d relayer transactions, %d reverted, from one $1 wallet", r.node.relayed(), reverted)
	}
}

// A use the chain records with no AuthorizationUsed log (cancelAuthorization)
// leaves the row "used, not located" forever: never failed, re-scanned from
// FromBlock to an ever-higher head on every pass.
func TestACanceledAuthorizationFails(t *testing.T) {
	r := newRig(t, false)
	sub := r.pay(2500, nil)
	r.node.set(func(n *node) { n.usdcOf[r.from] = new(big.Int) })
	if _, err := r.submit(sub); ReasonOf(err) != ReasonBalance {
		t.Fatalf("want refusal, got %v", err)
	}
	p, _ := sub.Authorization.Parse()
	r.node.set(func(n *node) { n.used[eip3009.Hex(p.From[:])+"|"+eip3009.Hex(p.Nonce[:])] = true }) // cancelAuthorization: state used, AuthorizationCanceled emitted, no AuthorizationUsed
	for i := 0; i < 200; i++ {                                                                      // ~4 days of passes
		r.node.mine(43_200 / 48)
		r.pass()
	}
	rec := r.stored(sub)
	t.Logf("after ~4 days: status=%s reason=%q fromBlock=%d head=%d", rec.Status, rec.Reason, rec.FromBlock, r.node.head)
	if rec.Open() {
		t.Errorf("STUCK: a canceled authorization is still %s; every pass scans %d blocks", rec.Status, int64(r.node.head)-rec.FromBlock)
	}
	_ = context.Background
}

// A payer who cancels the authorization (cancelAuthorization) leaves a nonce
// the token calls spent and no use: the AuthorizationCanceled event fails the
// payment as canceled at the next look, uncredited.
func TestACanceledAuthorizationFailsAtOnce(t *testing.T) {
	r := newRig(t, false)
	sub := r.pay(2500, nil)
	r.node.set(func(n *node) { n.usdcOf[r.from] = new(big.Int) })
	if _, err := r.submit(sub); ReasonOf(err) != ReasonBalance {
		t.Fatalf("want refusal, got %v", err)
	}
	r.node.cancel(sub)
	r.pass()
	if rec := r.stored(sub); rec.Status != remittance.Failed || rec.Reason != ReasonCanceled || r.credited() != 0 {
		t.Fatalf("%s %q, ledger %d", rec.Status, rec.Reason, r.credited())
	}
}

// A search for a use is at most 200 blocks a request, never reads a block
// twice, and ends with the authorization's window.
func TestTheSearchIsBoundedAndResumes(t *testing.T) {
	r := newRig(t, false)
	sub := r.pay(2500, nil)
	r.node.set(func(n *node) { n.usdcOf[r.from] = new(big.Int) })
	if _, err := r.submit(sub); ReasonOf(err) != ReasonBalance {
		t.Fatalf("want refusal, got %v", err)
	}
	p, _ := sub.Authorization.Parse()
	r.node.set(func(n *node) { n.used[eip3009.Hex(p.From[:])+"|"+eip3009.Hex(p.Nonce[:])] = true })
	for range 50 {
		r.node.mine(900)
		r.pass()
	}
	rec := r.stored(sub)
	window := rec.ToBlock - rec.FromBlock + 1
	var calls int
	r.node.set(func(n *node) { calls = n.logCalls })
	if rec.Status != remittance.Failed || rec.Reason != ReasonUnaccounted || rec.Scanned < rec.FromBlock {
		t.Fatalf("%s %q, scanned %d..%d of %d..%d", rec.Status, rec.Reason, rec.FromBlock, rec.Scanned, rec.FromBlock, rec.ToBlock)
	}
	if int64(calls) > window/logSpan+2 {
		t.Fatalf("%d log requests for a %d-block window", calls, window)
	}
}

// A credit waits for the chain's safe head, and from FinalCents its finalized
// head, however many confirmations have passed.
func TestACreditWaitsForTheSafeOrFinalizedHead(t *testing.T) {
	for _, tc := range []struct {
		cents int64
		lag   func(*node, uint64)
	}{
		{2500, func(n *node, v uint64) { n.safeLag = v }},
		{FinalCents, func(n *node, v uint64) { n.finalLag = v }},
	} {
		r := newRig(t, false)
		// The witness reads the same chain with its heads at the tip, so what
		// holds the credit is the node's own head, and then the witness's.
		eager := httptest.NewServer(tipped{r.node})
		t.Cleanup(eager.Close)
		r.chain.Witness = eager.URL
		r.s = New(Config{Testnet: true, Treasury: treasury, Relayer: r.s.cfg.Relayer, Chains: []Chain{r.chain}})
		r.s.now = func() time.Time { return r.now }
		r.node.set(func(n *node) { tc.lag(n, 40) })
		sub := r.pay(tc.cents, nil)
		if _, err := r.submit(sub); err != nil {
			t.Fatal(err)
		}
		r.node.mine(uint64(r.chain.Confirmations) + 10)
		r.step()
		if rec := r.stored(sub); rec.Credited || r.credited() != 0 {
			t.Fatalf("%d cents: credited above the head", tc.cents)
		}
		r.node.mine(40)
		r.step()
		if r.credited() != tc.cents {
			t.Fatalf("%d cents: ledger %d once past the head", tc.cents, r.credited())
		}
	}
}

// A second provider must show the same use: a witness that does not hold the
// credit back.
func TestAWitnessThatDisagreesHoldsTheCredit(t *testing.T) {
	r := newRig(t, false)
	other := newNode(r.chain.ChainID, r.chain.USDC, r.chain.Domain(), func() time.Time { return r.now })
	other.head = 1 << 40 // far past every block here: only its receipts can hold a credit back
	srv := httptest.NewServer(other)
	t.Cleanup(srv.Close)
	r.chain.Witness = srv.URL
	r.s = New(Config{Testnet: true, Treasury: treasury, Relayer: r.s.cfg.Relayer, Chains: []Chain{r.chain}})
	r.s.now = func() time.Time { return r.now }
	sub := r.pay(500, nil)
	if _, err := r.submit(sub); err != nil {
		t.Fatal(err)
	}
	r.settled()
	r.step()
	r.pass()
	if rec := r.stored(sub); rec.Credited || r.credited() != 0 || rec.Status != remittance.Confirming {
		t.Fatalf("credited on one provider's word: %s, ledger %d", rec.Status, r.credited())
	}
}

// A relayer transaction stuck in the pool is replaced at its own nonce with
// higher fees, never doubled at a new one; whichever lands is credited once.
func TestAStuckTransactionIsReplacedAtItsNonce(t *testing.T) {
	r := newRig(t, false)
	r.node.set(func(n *node) { n.hold = true })
	sub := r.pay(700, nil)
	if _, err := r.submit(sub); err != nil {
		t.Fatal(err)
	}
	first := r.stored(sub)
	r.now = r.now.Add(bumpAfter + time.Second)
	r.s.Reconcile(context.Background())
	second := r.stored(sub)
	if len(second.Sends) != 2 || second.RelayNonce != first.RelayNonce {
		t.Fatalf("sends %d, nonce %d then %d", len(second.Sends), first.RelayNonce, second.RelayNonce)
	}
	tip1, _ := new(big.Int).SetString(first.RelayTip, 10)
	tip2, _ := new(big.Int).SetString(second.RelayTip, 10)
	if tip2.Cmp(tip1) <= 0 {
		t.Fatalf("the replacement's tip %s is not above %s", tip2, tip1)
	}
	r.node.release()
	r.settled()
	r.step()
	if r.credited() != 700 || len(r.book.credits) != 1 {
		t.Fatalf("ledger %d", r.credited())
	}
}

// One wallet has one payment in flight: the next waits, then goes, and both
// are credited.
func TestAWalletHasOnePaymentInFlight(t *testing.T) {
	r := newRig(t, false)
	r.node.set(func(n *node) { n.usdcOf[r.from] = big.NewInt(1000 * 10_000); n.hold = true })
	a, b := r.pay(500, nil), r.pay(500, nil)
	for _, s := range []Submission{a, b} {
		if _, err := r.submit(s); err != nil {
			t.Fatal(err)
		}
	}
	if r.node.relayed() != 1 || len(r.stored(b).Sends) != 0 || r.stored(b).Status != remittance.Pending {
		t.Fatalf("relayed %d, second %+v", r.node.relayed(), r.stored(b))
	}
	r.node.release()
	r.step()
	r.node.set(func(n *node) { n.hold = false })
	r.step()
	r.settled()
	r.step()
	r.step()
	if r.credited() != 1000 || r.node.relayed() != 2 {
		t.Fatalf("ledger %d, relayed %d", r.credited(), r.node.relayed())
	}
}

// An org has at most MaxOpen payments pending, presents at most DayCents a
// day, and asks for at most PerMinute terms a minute.
func TestAnOrgIsBounded(t *testing.T) {
	r := newRig(t, false)
	r.node.set(func(n *node) { n.hold = true })
	for i := range MaxOpen {
		if _, err := r.submit(r.pay(100, nil)); err != nil {
			t.Fatalf("payment %d: %v", i, err)
		}
	}
	if _, err := r.submit(r.pay(100, nil)); ReasonOf(err) != ReasonOpen || StatusOf(err) != 429 {
		t.Fatalf("one past MaxOpen: %v", err)
	}

	d := newRig(t, false)
	d.node.fund(d.from, DayCents)
	for i := range DayCents / MaxCents {
		if _, err := d.submit(d.pay(MaxCents, nil)); err != nil {
			t.Fatalf("payment %d: %v", i, err)
		}
		d.step()
	}
	if _, err := d.submit(d.pay(100, nil)); ReasonOf(err) != ReasonDaily {
		t.Fatalf("past DayCents: %v", err)
	}

	q := newRig(t, false)
	for range PerMinute {
		q.terms()
	}
	if _, err := q.s.Terms(context.Background(), q.org, q.payer); ReasonOf(err) != ReasonRate || StatusOf(err) != 429 {
		t.Fatalf("past PerMinute: %v", err)
	}
	q.now = q.now.Add(time.Minute)
	q.terms()
}

// While another replica holds the relayer, nothing is drafted here; once its
// lease lapses, the payment goes.
func TestTheRelayerIsLeasedAcrossReplicas(t *testing.T) {
	r := newRig(t, false)
	if _, err := lock("relayer\x00"+r.chain.ID+"\x00"+r.relayer, func(l *remittance.Lock) bool {
		l.Holder, l.Until = "another-replica", r.now.Add(leaseFor)
		return true
	}); err != nil {
		t.Fatal(err)
	}
	sub := r.pay(300, nil)
	if _, err := r.submit(sub); err != nil {
		t.Fatal(err)
	}
	if r.node.relayed() != 0 {
		t.Fatalf("sent while another replica held the relayer")
	}
	r.now = r.now.Add(leaseFor + time.Minute)
	r.s.Reconcile(context.Background())
	if r.node.relayed() != 1 {
		t.Fatalf("not sent once the lease lapsed: %d", r.node.relayed())
	}
}

// A row the Circle Gateway rail left is never acted on: named once, counted
// for operators, and not looked at again.
func TestACircleGatewayRowIsLeftToOperators(t *testing.T) {
	r := newRig(t, false)
	old := remittance.New(systemDB())
	old.SetId("rem_circle")
	old.Org, old.Payer, old.Status, old.Network, old.AmountCents = r.org.Name, r.payer, remittance.Pending, r.chain.ID, 500
	old.Due = stamp(r.now)
	if _, err := old.Swap(); err != nil {
		t.Fatal(err)
	}
	r.pass()
	got, _ := load("rem_circle")
	if got.Due != never || got.Status != remittance.Pending || r.node.relayed() != 0 {
		t.Fatalf("%+v", got)
	}
	v, err := r.s.Treasury(context.Background())
	if err != nil || v.Legacy[remittance.Pending] != 1 || v.Open.Pending != 0 {
		t.Fatalf("treasury: %+v %v", v, err)
	}
}

// tipped is a node read through an endpoint whose safe and finalized heads are
// the latest block.
type tipped struct{ n *node }

func (t tipped) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req struct {
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	_ = json.Unmarshal(body, &req)
	if req.Method == "eth_getBlockByNumber" && len(req.Params) > 0 {
		body = bytes.Replace(body, []byte(`"safe"`), []byte(`"latest"`), 1)
		body = bytes.Replace(body, []byte(`"finalized"`), []byte(`"latest"`), 1)
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	t.n.ServeHTTP(w, r)
}

// A payment presented again whose record the Circle Gateway rail wrote is
// answered with that record and never relayed.
func TestACircleGatewayRowPresentedAgainIsNotRelayed(t *testing.T) {
	r := newRig(t, false)
	sub := r.pay(500, nil)
	c, err := verify(sub, r.s.cfg.Chains, treasury, r.org.Name, r.payer)
	if err != nil {
		t.Fatal(err)
	}
	old := remittance.New(systemDB())
	old.SetId(ID(c.chain.ID, c.auth.From, c.auth.Nonce))
	old.Org, old.Payer, old.Status, old.Network, old.Asset = r.org.Name, r.payer, remittance.Pending, c.chain.ID, c.chain.USDC
	old.From, old.To, old.Value = c.auth.From, c.auth.To, c.auth.Value
	old.ValidAfter, old.ValidBefore, old.Nonce, old.Salt, old.Signature = c.auth.ValidAfter, c.auth.ValidBefore, c.auth.Nonce, c.salt, c.sig
	old.Due = stamp(r.now.Add(time.Hour))
	if _, err := old.Swap(); err != nil {
		t.Fatal(err)
	}
	if v, err := r.submit(sub); err != nil || v.Status != remittance.Pending || r.node.relayed() != 0 {
		t.Fatalf("%+v %v, relayed %d", v, err, r.node.relayed())
	}
}

// The witness's own head holds a credit too: a node ahead of it is not enough.
func TestACreditWaitsForTheWitnessHead(t *testing.T) {
	r := newRig(t, false)
	eager := httptest.NewServer(tipped{r.node})
	t.Cleanup(eager.Close)
	r.chain.Witness, r.chain.RPC = r.chain.RPC, eager.URL // the node at the tip, the witness behind
	r.s = New(Config{Testnet: true, Treasury: treasury, Relayer: r.s.cfg.Relayer, Chains: []Chain{r.chain}})
	r.s.now = func() time.Time { return r.now }
	r.node.set(func(n *node) { n.safeLag = 40 })
	sub := r.pay(2500, nil)
	if _, err := r.submit(sub); err != nil {
		t.Fatal(err)
	}
	r.node.mine(uint64(r.chain.Confirmations) + 10)
	r.step()
	if r.credited() != 0 {
		t.Fatal("credited past the node's head and above the witness's")
	}
	r.node.mine(40)
	r.step()
	if r.credited() != 2500 {
		t.Fatalf("ledger %d once past both heads", r.credited())
	}
}
