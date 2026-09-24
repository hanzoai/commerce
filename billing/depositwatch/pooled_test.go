package depositwatch

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/models/cryptopaymentintent"
)

// The POOLED deposit model, which is XRPL's and nobody else's here.
//
// Every other chain in this rail mints one address per payer, so the address
// answers "whose money is this?". XRPL charges a non-refundable reserve for
// every funded account, so the model that is actually used is ONE account
// shared by every payer plus a per-deposit DESTINATION TAG. That inverts the
// matching, and everything below is about getting that inversion right —
// because the failure mode is not a missed deposit, it is CREDITING ONE
// CUSTOMER'S MONEY TO ANOTHER.

const (
	// The shared custody account. Note that it is the SAME string on every
	// intent below; that is the whole point.
	pooledAddr = "rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De"
	// xrplConfirmations must track the model's policy, like ethConfirmations.
	xrplConfirmations = 8
)

func xrplAsset() Asset {
	return Asset{
		Chain: "xrpl", Token: "rlusd",
		Contract: "RLUSD.rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De",
		RPCURL:   "http://rpc.test",
	}
}

// payer returns an intent sharing the pooled address, distinguished only by its
// routing tag.
func payer(id, subject, tag string) Watched {
	return Watched{
		Org: "acme", IntentID: id, Subject: subject,
		Address: pooledAddr, Tag: tag, Status: cryptopaymentintent.Pending,
	}
}

// rlusd returns a delivery of n whole RLUSD at xrplrpc's rendering scale (15).
func rlusd(n int64, tag string, block uint64, txHash string) Transfer {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(15), nil)
	return Transfer{
		To:    pooledAddr,
		Tag:   tag,
		Units: new(big.Int).Mul(big.NewInt(n), scale),
		// Canonical lowercase hex, the way billing/xrplrpc renders a hash.
		TxHash:     txHash,
		EventIndex: 0,
		Block:      block,
	}
}

// xrplReader answers about the pooled address, ignoring tags — exactly as the
// real client does, because the tag is the POLICY half's business.
func xrplReader(head uint64, transfers ...Transfer) *fakeReader {
	return &fakeReader{head: head, transfers: transfers, decimals: 15, symbol: "RLUSD"}
}

func confirmed(head uint64) uint64 { return head - xrplConfirmations + 1 }

// THE test. Three payers share one address; the tag decides who is credited,
// and it must be the right one.
func TestPooled_TheTagDecidesWhoIsCredited(t *testing.T) {
	head := uint64(1000)
	store := &fakeStore{watched: []Watched{
		payer("cpi_1", "acme/alice", "1001"),
		payer("cpi_2", "acme/bob", "1002"),
		payer("cpi_3", "acme/carol", "1003"),
	}}
	w := newWatcher(xrplAsset(), xrplReader(head,
		rlusd(25, "1002", confirmed(head), "aa11"),
	), store, running(head, "xrpl:rlusd"))

	n, err := w.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if n != 1 {
		t.Fatalf("credited %d deposits, want 1", n)
	}
	got := store.credits[0]
	if got.Subject != "acme/bob" || got.IntentID != "cpi_2" {
		t.Fatalf("tag 1002 credited %s (%s) — the deposit went to the wrong customer", got.Subject, got.IntentID)
	}
	if got.AmountCents != 2500 {
		t.Fatalf("25 RLUSD credited as %d cents", got.AmountCents)
	}
	if len(store.unattributed) != 0 {
		t.Fatalf("a correctly tagged deposit was ALSO recorded as unattributed: %+v", store.unattributed)
	}
}

// A tag we never issued names nobody. It must not be credited to anyone — and
// it must not vanish either, because it is somebody's real money.
func TestPooled_AnUnknownTagIsRecordedAndCreditedToNobody(t *testing.T) {
	head := uint64(1000)
	store := &fakeStore{watched: []Watched{payer("cpi_1", "acme/alice", "1001")}}
	w := newWatcher(xrplAsset(), xrplReader(head,
		rlusd(500, "999999", confirmed(head), "bb22"),
	), store, running(head, "xrpl:rlusd"))

	n, err := w.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if n != 0 || len(store.credits) != 0 {
		t.Fatalf("an unknown tag credited %d deposit(s): %+v", n, store.credits)
	}
	if len(store.unattributed) != 1 {
		t.Fatalf("recorded %d unattributed deposits, want 1 — the money vanished", len(store.unattributed))
	}
	u := store.unattributed[0]
	if u.Tag != "999999" || u.Address != pooledAddr || u.TxHash != "bb22" {
		t.Fatalf("the record does not describe the payment: %+v", u)
	}
	if u.Units != "500000000000000000" {
		t.Fatalf("the record says %s units, want 500000000000000000 — an operator cannot refund what it cannot size", u.Units)
	}
	if u.DedupKey != "xrpl:bb22:0" {
		t.Fatalf("dedup key %q", u.DedupKey)
	}
}

// A payment with NO destination tag at all. Same answer, and it is a DIFFERENT
// case from tag "0" — see the test below.
func TestPooled_AnUntaggedPaymentIsRecordedAndCreditedToNobody(t *testing.T) {
	head := uint64(1000)
	store := &fakeStore{watched: []Watched{payer("cpi_1", "acme/alice", "1001")}}
	w := newWatcher(xrplAsset(), xrplReader(head,
		rlusd(7, "", confirmed(head), "cc33"),
	), store, running(head, "xrpl:rlusd"))

	if n, err := w.Sync(context.Background()); err != nil || n != 0 {
		t.Fatalf("Sync = %d, %v — an untagged payment was credited", n, err)
	}
	if len(store.unattributed) != 1 {
		t.Fatalf("an untagged payment was silently dropped (%d records)", len(store.unattributed))
	}
	if store.unattributed[0].Tag != "" {
		t.Fatalf("recorded tag %q, want the empty string so an operator can see it carried none", store.unattributed[0].Tag)
	}
}

// Tag "0" is a LEGAL tag a payer may hold. An untagged payment must not be
// credited to whoever was issued it.
func TestPooled_TagZeroIsNotTheAbsenceOfATag(t *testing.T) {
	head := uint64(1000)
	store := &fakeStore{watched: []Watched{payer("cpi_zero", "acme/zeta", "0")}}
	w := newWatcher(xrplAsset(), xrplReader(head,
		rlusd(11, "", confirmed(head), "dd44"), // NO tag
	), store, running(head, "xrpl:rlusd"))

	if _, err := w.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(store.credits) != 0 {
		t.Fatalf("an untagged payment was credited to the holder of tag 0: %+v", store.credits)
	}
	if len(store.unattributed) != 1 {
		t.Fatal("an untagged payment was neither credited nor recorded")
	}

	// ...and the holder of tag 0 IS credited by a payment that really carries 0.
	store2 := &fakeStore{watched: []Watched{payer("cpi_zero", "acme/zeta", "0")}}
	w2 := newWatcher(xrplAsset(), xrplReader(head,
		rlusd(11, "0", confirmed(head), "dd45"),
	), store2, running(head, "xrpl:rlusd"))
	if _, err := w2.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(store2.credits) != 1 || store2.credits[0].Subject != "acme/zeta" {
		t.Fatalf("tag 0 did not credit its holder: %+v", store2.credits)
	}
}

// An unattributable payment must NOT wedge the rail. The pooled address is
// published, so anyone can send one drop to it with no tag; if that stopped the
// pass, a stranger could deny every other customer their deposits.
func TestPooled_AnUnattributedPaymentDoesNotStopTheOthers(t *testing.T) {
	head := uint64(1000)
	cursor := running(head, "xrpl:rlusd")
	before, _ := cursor.Last(context.Background(), "xrpl:rlusd")
	store := &fakeStore{watched: []Watched{
		payer("cpi_1", "acme/alice", "1001"),
		payer("cpi_2", "acme/bob", "1002"),
	}}
	w := newWatcher(xrplAsset(), xrplReader(head,
		rlusd(1, "", confirmed(head), "ee55"),      // nobody's
		rlusd(30, "1001", confirmed(head), "ee56"), // alice's
		rlusd(40, "1002", confirmed(head), "ee57"), // bob's
	), store, cursor)

	n, err := w.Sync(context.Background())
	if err != nil {
		t.Fatalf("one stranger's untagged payment stopped the whole pass: %v", err)
	}
	if n != 2 {
		t.Fatalf("credited %d deposits, want 2 (alice and bob)", n)
	}
	if store.totalCents() != 7000 {
		t.Fatalf("credited %d cents, want 7000", store.totalCents())
	}
	after, _ := cursor.Last(context.Background(), "xrpl:rlusd")
	if after <= before {
		t.Fatalf("the cursor did not advance past an unattributed payment (%d → %d) — the rail is wedged", before, after)
	}
}

// A record that cannot be WRITTEN is different: it must park the cursor, because
// letting the scan move on would destroy the only evidence the money exists.
func TestPooled_AFailedRecordParksTheCursor(t *testing.T) {
	head := uint64(1000)
	cursor := running(head, "xrpl:rlusd")
	before, _ := cursor.Last(context.Background(), "xrpl:rlusd")
	store := &fakeStore{
		watched:     []Watched{payer("cpi_1", "acme/alice", "1001")},
		unattribErr: errors.New("datastore down"),
	}
	w := newWatcher(xrplAsset(), xrplReader(head,
		rlusd(9, "nope", confirmed(head), "ff66"),
	), store, cursor)

	if _, err := w.Sync(context.Background()); err == nil {
		t.Fatal("a failed unattributed record was swallowed")
	}
	after, _ := cursor.Last(context.Background(), "xrpl:rlusd")
	if after != before {
		t.Fatalf("the cursor advanced (%d → %d) past a payment nothing recorded — it can never be seen again", before, after)
	}
}

// Shallow first, deep later. An unattributed payment is recorded only once it is
// as deep as a CREDIT would need to be, so the record describes the canonical
// chain rather than a transaction that may still be reorganised away.
func TestPooled_AnUnattributedPaymentIsRecordedOnlyOnceConfirmed(t *testing.T) {
	head := uint64(1000)
	shallow := rlusd(3, "nope", head, "aa77") // depth 1
	store := &fakeStore{watched: []Watched{payer("cpi_1", "acme/alice", "1001")}}
	w := newWatcher(xrplAsset(), xrplReader(head, shallow), store, running(head, "xrpl:rlusd"))
	if _, err := w.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(store.unattributed) != 0 {
		t.Fatalf("recorded an unattributed payment at depth 1: %+v", store.unattributed)
	}
	if len(store.sights) != 0 {
		t.Fatalf("a payment belonging to nobody was sighted onto an intent: %+v", store.sights)
	}

	// The same payment, now buried deep enough.
	deep := head + xrplConfirmations
	store2 := &fakeStore{watched: []Watched{payer("cpi_1", "acme/alice", "1001")}}
	w2 := newWatcher(xrplAsset(), xrplReader(deep, shallow), store2, running(deep, "xrpl:rlusd"))
	if _, err := w2.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(store2.unattributed) != 1 {
		t.Fatalf("a confirmed unattributable payment was not recorded (%d records)", len(store2.unattributed))
	}
}

// Two intents holding the SAME tag on the same address is the pooled version of
// two intents claiming one address: nobody can say whose money it is, so the
// asset stops rather than guessing with it.
func TestPooled_TwoIntentsWithOneTagFailClosed(t *testing.T) {
	head := uint64(1000)
	cursor := running(head, "xrpl:rlusd")
	before, _ := cursor.Last(context.Background(), "xrpl:rlusd")
	store := &fakeStore{watched: []Watched{
		payer("cpi_1", "acme/alice", "1001"),
		payer("cpi_2", "acme/bob", "1001"), // the same tag
	}}
	w := newWatcher(xrplAsset(), xrplReader(head,
		rlusd(25, "1001", confirmed(head), "bb88"),
	), store, cursor)

	_, err := w.Sync(context.Background())
	if err == nil {
		t.Fatal("two intents holding one tag did not stop the asset")
	}
	if !strings.Contains(err.Error(), "more than one intent") {
		t.Fatalf("error %q does not explain the collision", err)
	}
	if len(store.credits) != 0 {
		t.Fatalf("credited a deposit whose owner is ambiguous: %+v", store.credits)
	}
	if after, _ := cursor.Last(context.Background(), "xrpl:rlusd"); after != before {
		t.Fatal("the cursor advanced past a window that was never scanned")
	}
}

// Many intents, ONE address. The chain must be asked about the address once —
// that is the entire economic point of the pooled model, and a watcher that
// asked per intent would make a 10,000-customer deploy send 10,000 queries.
func TestPooled_ManyIntentsAreOneWatchedAddress(t *testing.T) {
	head := uint64(1000)
	var watched []Watched
	for i := 0; i < 250; i++ {
		watched = append(watched, payer(
			"cpi_"+strings.Repeat("x", i%3)+itoa(i),
			"acme/user"+itoa(i),
			itoa(2000+i),
		))
	}
	reader := xrplReader(head, rlusd(5, "2100", confirmed(head), "cc99"))
	store := &fakeStore{watched: watched}
	w := newWatcher(xrplAsset(), reader, store, running(head, "xrpl:rlusd"))

	if _, err := w.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if reader.maxAddrsSeen != 1 {
		t.Fatalf("the reader was asked about %d addresses for %d intents, want 1 — the pooled model was not used", reader.maxAddrsSeen, len(watched))
	}
	if len(store.credits) != 1 || store.credits[0].Subject != "acme/user100" {
		t.Fatalf("tag 2100 credited %+v", store.credits)
	}
}

// The dedup key names the ON-CHAIN EVENT and must not contain the routing tag.
// A key that did would produce a second ledger row for one payment if the same
// payment were ever re-read against a corrected tag.
func TestPooled_TheDedupKeyDoesNotContainTheTag(t *testing.T) {
	a := &asset{Asset: xrplAsset()}
	tagged := a.dedupKey(rlusd(1, "1001", 900, "dd00"))
	untagged := a.dedupKey(rlusd(1, "", 900, "dd00"))
	if tagged != untagged {
		t.Fatalf("the dedup key changes with the tag: %q vs %q", tagged, untagged)
	}
	if tagged != "xrpl:dd00:0" {
		t.Fatalf("dedup key = %q, want xrpl:dd00:0", tagged)
	}
}

// Identity is the ONE place address and tag are combined, and it must behave
// differently per chain — the whole reason it exists rather than a comparison
// on Watched.Address.
func TestIdentity_IsPooledOnlyWhereItShouldBe(t *testing.T) {
	x := xrplAsset()
	if !x.Pooled() {
		t.Fatal("XRPL is not reported as pooled")
	}
	if got := x.Identity(pooledAddr, "1001"); got == x.Identity(pooledAddr, "1002") {
		t.Fatalf("two tags on one address share an identity (%q)", got)
	}
	if x.Identity(pooledAddr, "") == x.Identity(pooledAddr, "0") {
		t.Fatal("no tag and tag 0 share an identity")
	}

	for _, a := range []Asset{
		{Chain: "base", Token: "usdc"},
		{Chain: "solana", Token: "usdc"},
		{Chain: "ton", Token: "usdt"},
	} {
		if a.Pooled() {
			t.Fatalf("%s is reported as pooled; it mints one address per payer", a.Chain)
		}
		// A tag must be IGNORED on a chain that does not route by one, so a
		// stray value in the field cannot make a real deposit unmatchable.
		if a.Identity("Addr1", "") != a.Identity("Addr1", "12345") {
			t.Fatalf("%s: a tag changed the identity of a per-payer address", a.Chain)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
