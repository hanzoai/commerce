// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/luxfi/crypto"

	"github.com/hanzoai/commerce/billing/bucket"
	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/remittance"
	txutil "github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/payment/eip3009"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The rig: a real datastore (util/test/ae), a fake chain carrying USDC that
// keeps the token's own rules (node), and a fake host ledger that dedups on
// the idempotency key the way the real one does. Exactly-once is a property of
// keys meeting a store that upserts on them, so only a real store can prove it.

const treasury = "0x7e57000000000000000000000000000000000001"

// book is the host ledger, keyed as the real one is.
type book struct {
	mu      sync.Mutex
	credits map[string]creditledger.CreditInput
	calls   int // Credit calls, replays included
	broken  bool
}

func newBook() *book { return &book{credits: map[string]creditledger.CreditInput{}} }

func (b *book) Credit(_ context.Context, in creditledger.CreditInput) (string, int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.broken {
		return "", 0, errors.New("ledger unavailable")
	}
	b.calls++
	scope := fmt.Sprintf("%s|%t|%s", in.Org, in.Test, in.IdempotencyKey)
	if _, ok := b.credits[scope]; !ok {
		b.credits[scope] = in
	}
	return "post_" + scope, 0, nil
}

func (b *book) Balance(_ context.Context, org, subject, _ string, test bool) (int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var n int64
	for _, c := range b.credits {
		if c.Org == org && c.Subject == subject && c.Test == test {
			n += c.AmountCents
		}
	}
	return n, nil
}

func (b *book) Debit(context.Context, creditledger.DebitInput) (string, int64, error) {
	return "", 0, errors.New("not used by this rail")
}

func (b *book) Refund(context.Context, creditledger.RefundInput) (string, error) {
	return "", errors.New("not used by this rail")
}

func (b *book) Restore(context.Context, creditledger.ClawbackInput) (string, error) {
	return "", errors.New("not used by this rail")
}

func (b *book) Clawback(context.Context, creditledger.ClawbackInput) (string, error) {
	return "", errors.New("this rail never takes a credit back")
}

func (b *book) set(fn func(*book)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	fn(b)
}

// rig is one test's world: the rail, the chain, the host ledger, an org and a
// payer with a wallet. The clock is the rail's and the chain's, and stands
// still unless a test moves it.
type rig struct {
	t       *testing.T
	s       *Service
	node    *node
	book    *book
	chain   Chain
	org     *organization.Organization
	payer   string
	key     *ecdsa.PrivateKey
	from    string
	relayer string
	now     time.Time
}

var orgs int

// newRig is a testnet rail (TEST books) on Base Sepolia's table row, unless
// live, which is the mainnet rail on Base's.
func newRig(t *testing.T, live bool) *rig {
	t.Helper()
	c := ae.NewContext()
	t.Cleanup(c.Close)
	b := newBook()
	creditledger.Set(b)
	t.Cleanup(func() { creditledger.Set(nil) })

	orgs++
	name := fmt.Sprintf("acme%d%d", time.Now().UnixNano()%1_000_000, orgs)
	org := &organization.Organization{}
	org.Name, org.Live = name, live

	r := &rig{t: t, book: b, org: org, payer: name, now: time.Unix(time.Now().Unix(), 0)}
	r.key = mustKey(t)
	r.from = eip3009.Hex(crypto.PubkeyToAddress(r.key.PublicKey).Bytes())
	relayer := mustKey(t)
	r.relayer = eip3009.Hex(crypto.PubkeyToAddress(relayer.PublicKey).Bytes())

	r.chain = testnet[0]
	if live {
		r.chain = mainnet[0]
	}
	r.node = newNode(r.chain.ChainID, r.chain.USDC, r.chain.Domain(), func() time.Time { return r.now })
	srv := httptest.NewServer(r.node)
	t.Cleanup(srv.Close)
	r.chain.RPC = srv.URL
	// The witness is a second endpoint onto the same chain: it agrees unless a
	// test points it somewhere else.
	witness := httptest.NewServer(r.node)
	t.Cleanup(witness.Close)
	r.chain.Witness = witness.URL

	r.s = New(Config{Testnet: !live, Treasury: treasury, Relayer: relayer, Chains: []Chain{r.chain}})
	r.s.now = func() time.Time { return r.now }
	r.node.fund(r.from, 100_000)
	r.node.gas(r.relayer, big.NewInt(1e18))
	return r
}

func mustKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// terms asks the rail for terms as this rig's caller.
func (r *rig) terms() Terms {
	r.t.Helper()
	t, err := r.s.Terms(context.Background(), r.org, r.payer)
	if err != nil {
		r.t.Fatalf("terms: %v", err)
	}
	return t
}

// pay signs a payment of cents against fresh terms, letting a test change the
// authorization before it is signed.
func (r *rig) pay(cents int64, edit func(*eip3009.Authorization)) Submission {
	r.t.Helper()
	return r.sign(r.terms(), cents, edit)
}

func (r *rig) sign(t Terms, cents int64, edit func(*eip3009.Authorization)) Submission {
	r.t.Helper()
	acc := t.Accepts[0]
	a := eip3009.Authorization{
		From:        r.from,
		To:          t.PayTo,
		Value:       new(big.Int).Mul(big.NewInt(cents), big.NewInt(10_000)).String(),
		ValidAfter:  strconv.FormatInt(r.now.Add(-time.Minute).Unix(), 10),
		ValidBefore: strconv.FormatInt(r.now.Add(time.Hour).Unix(), 10),
		Nonce:       t.Nonce,
	}
	if edit != nil {
		edit(&a)
	}
	return Submission{Network: acc.Network, Salt: t.Salt, Authorization: a, Signature: signWith(r.t, r.key, acc.Domain, a)}
}

func signWith(t *testing.T, key *ecdsa.PrivateKey, d eip3009.Domain, a eip3009.Authorization) string {
	t.Helper()
	p, err := a.Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	digest, err := d.Digest(p)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	sig, err := crypto.Sign(digest[:], key)
	if err != nil {
		t.Fatal(err)
	}
	return eip3009.Hex(sig) // v as 0/1: the rail must canonicalise it for the token
}

func (r *rig) submit(sub Submission) (View, error) {
	return r.s.Submit(context.Background(), r.org, r.payer, sub)
}

// settled mines past the point a payment of any size is credited: its depth,
// and the finalized head.
func (r *rig) settled() {
	r.node.mine(uint64(r.chain.Confirmations) + r.node.finalLag + 1)
}

// pass moves the clock past any wait the reconciler set and runs one pass.
func (r *rig) pass() {
	r.now = r.now.Add(31 * time.Minute)
	r.s.Reconcile(context.Background())
}

// step moves the clock past the resend wait and runs one pass.
func (r *rig) step() {
	r.now = r.now.Add(resend + time.Second)
	r.s.Reconcile(context.Background())
}

// stored reads the remittance a submission made.
func (r *rig) stored(sub Submission) *remittance.Remittance {
	r.t.Helper()
	rec, err := load(ID(sub.Network, sub.Authorization.From, sub.Authorization.Nonce))
	if err != nil {
		r.t.Fatalf("load remittance: %v", err)
	}
	return rec
}

func (r *rig) exists(sub Submission) bool {
	_, err := load(ID(sub.Network, sub.Authorization.From, sub.Authorization.Nonce))
	return err == nil
}

// credited is what the host ledger holds for this caller.
func (r *rig) credited() int64 {
	r.t.Helper()
	n, _ := r.book.Balance(context.Background(), strings.ToLower(r.org.Name), r.payer, "usd", r.org.TestMode())
	return n
}

// receipts is commerce's own ledger for this caller, as the balance read
// computes it.
func (r *rig) receipts() bucket.Split {
	r.t.Helper()
	raw, err := txutil.GetRawByCurrency(orgDB(r.org.Name).Context, r.payer, "iam-user", "usd", r.org.TestMode())
	if err != nil {
		r.t.Fatalf("read receipts: %v", err)
	}
	return bucket.Compute(raw, r.payer, time.Now())
}

func random32() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	return eip3009.Hex(b[:])
}
