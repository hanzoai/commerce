// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
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
	"github.com/hanzoai/commerce/payment/providers/circle"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The rig: a real datastore (util/test/ae), a fake Circle Gateway that keeps
// Gateway's own rules — it verifies the EIP-712 signature with the same digest
// a payer signs, refuses a payee or amount that is not the requirement's, holds
// each payer's balance and LOCKS what it accepts, and refuses a second use of
// a (payer, nonce) — and a fake host ledger that dedups on the idempotency key
// the way the real one does. Exactly-once is a property of keys meeting a store
// that upserts on them, so only a real store can prove it.

const (
	treasury = "0x7e57000000000000000000000000000000000001"
	caip     = "eip155:84532"
	usdc     = "0x036cbd53842c5426634e7929541ec2318f3dcf7e"
	wallet   = "0x0077777d7eba4688bdef3e311b846f25870a19b9"
	week     = 7 * 24 * time.Hour
)

// fake is Circle Gateway, as far as this rail can observe it.
type fake struct {
	mu       sync.Mutex
	kinds    []circle.Kind
	held     map[string]int64 // payer → available base units
	payments map[string]*circle.Payment
	byNonce  map[string]string // network|from|nonce → payment id
	settles  int
	finds    int

	// Knobs a test turns.
	lost    int           // accept the payment, then answer with this HTTP status
	down    int           // answer with this HTTP status and do nothing
	locked  chan struct{} // closed once a payment is LOCKED, before the answer
	release chan struct{} // the answer waits for this, when set
	reason  string        // refuse every settle with this errorReason
	hide    int           // hide accepted payments from Find for this many calls
	refused []string      // errorReasons answered, in order
}

func newFake() *fake {
	return &fake{
		kinds: []circle.Kind{{
			Version: 2, Scheme: "exact", Network: caip,
			Extra: circle.Extra{
				Name: circle.Name, Version: circle.Release, VerifyingContract: wallet, MinValidity: int64(week / time.Second),
				Assets: []circle.Asset{{Symbol: "USDC", Address: usdc, Decimals: 6, Versions: []string{"1", "2"}}},
			},
		}},
		held:     map[string]int64{},
		payments: map[string]*circle.Payment{},
		byNonce:  map[string]string{},
	}
}

func (f *fake) fund(addr string, cents int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.held[strings.ToLower(addr)] += cents * 10_000
}

func (f *fake) count() (settles, payments int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.settles, len(f.payments)
}

func (f *fake) set(fn func(*fake)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

func (f *fake) payment(id string) circle.Payment {
	f.mu.Lock()
	defer f.mu.Unlock()
	return *f.payments[id]
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reply := func(status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/x402/supported":
		f.mu.Lock()
		kinds := f.kinds
		f.mu.Unlock()
		reply(200, map[string]any{"kinds": kinds})
	case r.Method == http.MethodPost && r.URL.Path == "/v1/x402/settle":
		f.settle(w, r, reply)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/x402/transfers":
		q := r.URL.Query()
		f.mu.Lock()
		f.finds++
		out := []circle.Payment{}
		if f.hide > 0 {
			f.hide--
		} else if id, ok := f.byNonce[q.Get("network")+"|"+strings.ToLower(q.Get("from"))+"|"+strings.ToLower(q.Get("nonce"))]; ok {
			out = append(out, *f.payments[id])
		}
		f.mu.Unlock()
		reply(200, map[string]any{"transfers": out})
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/x402/transfers/"):
		f.mu.Lock()
		p, ok := f.payments[strings.TrimPrefix(r.URL.Path, "/v1/x402/transfers/")]
		var cp circle.Payment
		if ok {
			cp = *p
		}
		f.mu.Unlock()
		if !ok {
			reply(404, map[string]any{"success": false, "message": "Transfer not found"})
			return
		}
		reply(200, cp)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/info":
		reply(200, circle.Info{Version: 1, Domains: []circle.Chain{{Chain: "Base", Network: "Sepolia", Domain: 6}, {Chain: "Ethereum", Network: "Sepolia", Domain: 0}}})
	case r.Method == http.MethodPost && r.URL.Path == "/v1/balances":
		reply(200, map[string]any{"token": "USDC", "balances": []circle.Balance{
			{Domain: 6, Depositor: treasury, Balance: "12.500000"},
			{Domain: 0, Depositor: treasury, Balance: "0.000001"},
		}})
	case r.Method == http.MethodPost && r.URL.Path == "/v1/deposits":
		reply(200, map[string]any{"token": "USDC", "deposits": []circle.Deposit{
			{Depositor: treasury, Domain: 6, TransactionHash: "0x" + strings.Repeat("ab", 32), Amount: "2500000", Status: "pending"},
		}})
	default:
		reply(404, map[string]any{"code": 404, "message": "no route " + r.URL.Path})
	}
}

func (f *fake) settle(w http.ResponseWriter, r *http.Request, reply func(int, any)) {
	var in struct {
		Payment struct {
			Version  int                 `json:"x402Version"`
			Resource circle.Resource     `json:"resource"`
			Accepted circle.Requirements `json:"accepted"`
			Payload  circle.Payload      `json:"payload"`
		} `json:"paymentPayload"`
		Req circle.Requirements `json:"paymentRequirements"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Payment.Resource.URL == "" || in.Payment.Resource.MimeType == "" {
		reply(400, map[string]any{"message": "Invalid request"})
		return
	}
	f.mu.Lock()
	f.settles++
	if f.down != 0 {
		down := f.down
		f.mu.Unlock()
		reply(down, map[string]any{"code": down, "message": "unavailable"})
		return
	}
	refuse := func(reason string) {
		f.refused = append(f.refused, reason)
		f.mu.Unlock()
		reply(200, circle.Settlement{Success: false, Reason: reason, Network: in.Req.Network})
	}
	if f.reason != "" {
		refuse(f.reason)
		return
	}
	a, req := in.Payment.Payload.Authorization, in.Req
	if !strings.EqualFold(a.To, req.PayTo) {
		refuse("address_mismatch")
		return
	}
	if a.Value != req.Amount {
		refuse("amount_mismatch")
		return
	}
	p, err := a.Parse()
	if err != nil {
		refuse("invalid_payload")
		return
	}
	chain, _ := chainID(req.Network)
	d, _ := circle.Domain{Name: req.Extra.Name, Version: req.Extra.Version, ChainID: chain, VerifyingContract: req.Extra.VerifyingContract}.Digest(p)
	sig := in.Payment.Payload.Signature
	if b, _ := strconv.ParseUint(sig[len(sig)-2:], 16, 8); b < 27 {
		refuse("invalid_signature") // OpenZeppelin's ECDSA.recover wants v = 27/28
		return
	}
	if who, err := circle.Recover(d, sig); err != nil || circle.Hex(who[:]) != strings.ToLower(a.From) {
		refuse("invalid_signature")
		return
	}
	key := req.Network + "|" + strings.ToLower(a.From) + "|" + strings.ToLower(a.Nonce)
	if _, used := f.byNonce[key]; used {
		refuse("nonce_already_used")
		return
	}
	value := p.Value.Int64()
	if f.held[strings.ToLower(a.From)] < value {
		refuse("insufficient_balance")
		return
	}
	f.held[strings.ToLower(a.From)] -= value // LOCKED
	var u [16]byte
	_, _ = rand.Read(u[:])
	h := hex.EncodeToString(u[:])
	id := h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
	f.payments[id] = &circle.Payment{
		ID: id, Status: circle.Received, Token: "USDC", SendingNetwork: req.Network, RecipientNetwork: req.Network,
		From: strings.ToLower(a.From), To: strings.ToLower(a.To), Amount: a.Value, Nonce: strings.ToLower(a.Nonce),
	}
	f.byNonce[key] = id
	lost, locked, release := f.lost, f.locked, f.release
	f.mu.Unlock()
	if locked != nil {
		close(locked)
	}
	if release != nil {
		<-release
	}
	if lost != 0 {
		reply(lost, map[string]any{"success": false, "errorReason": "unexpected_error", "transaction": "", "network": req.Network})
		return
	}
	reply(200, circle.Settlement{Success: true, Payer: strings.ToLower(a.From), Transaction: id, Network: req.Network})
}

// mark moves a Gateway payment along (the batch landed, or failed).
func (f *fake) mark(id, status string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.payments[id].Status = status
	if status == circle.Confirmed {
		f.payments[id].TxHash = "0x" + strings.Repeat("cd", 32)
	}
}

// book is the host ledger, keyed as the real one is.
type book struct {
	mu        sync.Mutex
	credits   map[string]creditledger.CreditInput
	clawbacks map[string]creditledger.ClawbackInput
	calls     int // Clawback calls, replays included
	broken    bool
}

func newBook() *book {
	return &book{credits: map[string]creditledger.CreditInput{}, clawbacks: map[string]creditledger.ClawbackInput{}}
}

func (b *book) Credit(_ context.Context, in creditledger.CreditInput) (string, int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.broken {
		return "", 0, errors.New("ledger unavailable")
	}
	scope := fmt.Sprintf("%s|%t|%s", in.Org, in.Test, in.IdempotencyKey)
	if _, ok := b.credits[scope]; !ok {
		b.credits[scope] = in
	}
	return "post_" + scope, 0, nil
}

func (b *book) Balance(_ context.Context, org, subject, cur string, test bool) (int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var n int64
	for _, c := range b.credits {
		if c.Org == org && c.Subject == subject && c.Test == test {
			n += c.AmountCents
		}
	}
	for _, c := range b.clawbacks {
		if c.Org == org && c.Subject == subject && c.Test == test {
			n -= c.AmountCents
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

func (b *book) Clawback(_ context.Context, in creditledger.ClawbackInput) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.broken {
		return "", errors.New("ledger unavailable")
	}
	b.calls++
	scope := fmt.Sprintf("%s|%t|%s", in.Org, in.Test, in.Ref)
	if _, ok := b.clawbacks[scope]; !ok {
		b.clawbacks[scope] = in
	}
	return "claw_" + scope, nil
}

func (b *book) posted() []creditledger.CreditInput {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := []creditledger.CreditInput{}
	for _, c := range b.credits {
		out = append(out, c)
	}
	return out
}

func (b *book) taken() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.clawbacks)
}

// rig is one test's world: the rail, Circle, the host ledger, an org and a
// payer with a wallet. The clock is the rail's own and stands still unless a
// test moves it.
type rig struct {
	t      *testing.T
	s      *Service
	gw     *fake
	book   *book
	org    *organization.Organization
	payer  string
	key    *ecdsa.PrivateKey
	from   string
	server *httptest.Server
	now    time.Time
}

var orgs int

func newRig(t *testing.T, n circle.Network, live bool) *rig {
	t.Helper()
	c := ae.NewContext()
	t.Cleanup(c.Close)
	gw := newFake()
	srv := httptest.NewServer(gw)
	t.Cleanup(srv.Close)
	b := newBook()
	creditledger.Set(b)
	t.Cleanup(func() { creditledger.Set(nil) })

	orgs++
	name := fmt.Sprintf("acme%d%d", time.Now().UnixNano()%1_000_000, orgs)
	org := &organization.Organization{}
	org.Name, org.Live = name, live

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{
		t: t, gw: gw, book: b, org: org, payer: name, key: key, server: srv,
		from: circle.Hex(crypto.PubkeyToAddress(key.PublicKey).Bytes()),
		now:  time.Unix(time.Now().Unix(), 0),
	}
	r.s = New(circle.Config{Network: n, Treasury: treasury}, circle.New(srv.URL))
	r.s.now = func() time.Time { return r.now }
	gw.fund(r.from, 1_000_000)
	return r
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
func (r *rig) pay(cents int64, edit func(*circle.Authorization)) Submission {
	r.t.Helper()
	t := r.terms()
	return r.sign(t, cents, edit)
}

func (r *rig) sign(t Terms, cents int64, edit func(*circle.Authorization)) Submission {
	r.t.Helper()
	acc := t.Accepts[0]
	a := circle.Authorization{
		From:        r.from,
		To:          t.PayTo,
		Value:       new(big.Int).Mul(big.NewInt(cents), big.NewInt(10_000)).String(),
		ValidAfter:  strconv.FormatInt(r.now.Add(-10*time.Minute).Unix(), 10),
		ValidBefore: strconv.FormatInt(r.now.Add(week+time.Hour).Unix(), 10),
		Nonce:       t.Nonce,
	}
	if edit != nil {
		edit(&a)
	}
	return Submission{Network: acc.Network, Salt: t.Salt, Authorization: a, Signature: r.signWith(r.key, acc.Domain, a)}
}

func (r *rig) signWith(key *ecdsa.PrivateKey, d circle.Domain, a circle.Authorization) string {
	r.t.Helper()
	p, err := a.Parse()
	if err != nil {
		r.t.Fatalf("parse: %v", err)
	}
	digest, err := d.Digest(p)
	if err != nil {
		r.t.Fatalf("digest: %v", err)
	}
	sig, err := crypto.Sign(digest[:], key)
	if err != nil {
		r.t.Fatal(err)
	}
	return circle.Hex(sig) // v as 0/1: the rail must canonicalise it for Gateway
}

// pass moves the clock past any wait the reconciler set and runs one pass.
func (r *rig) pass() {
	r.now = r.now.Add(31 * time.Minute)
	r.s.Reconcile(context.Background())
}

func (r *rig) submit(sub Submission) (View, error) {
	return r.s.Submit(context.Background(), r.org, r.payer, sub)
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
	return circle.Hex(b[:])
}
