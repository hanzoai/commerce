package depositledger

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/util/test/ae"
)

// ledger is the host's spendable ledger as commerce sees it when embedded: it
// dedups on the idempotency key within one (org, test) ledger, the way the real
// host does. Only Credit is used here; the embedded interface answers nothing else.
type ledger struct {
	creditledger.CreditLedger
	mu     sync.Mutex
	fail   error
	seen   map[string]bool
	posted []creditledger.CreditInput
}

func (l *ledger) Credit(_ context.Context, in creditledger.CreditInput) (string, int64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.fail != nil {
		return "", 0, l.fail
	}
	key := fmt.Sprintf("%s|%t|%s", in.Org, in.Test, in.IdempotencyKey)
	if !l.seen[key] {
		l.seen[key] = true
		l.posted = append(l.posted, in)
	}
	return "tx", 0, nil
}

func inject(t *testing.T) *ledger {
	t.Helper()
	l := &ledger{seen: map[string]bool{}}
	creditledger.Set(l)
	t.Cleanup(func() { creditledger.Set(nil) })
	return l
}

func deposit(org, subject string) depositwatch.Credit {
	return depositwatch.Credit{
		Org: org, Subject: subject, Chain: "ethereum", Token: "usdc",
		AmountCents: 2550, Units: "25500000", PegRate: "1.00",
		TxHash: "0xfeedface", EventIndex: 4, Block: 980, Confirmations: 21,
		DedupKey: "ethereum:0xfeedface:4",
	}
}

// Embedded in a host, the balance a customer spends is the host's ledger. A
// confirmed deposit must fund it, once, as cash, at the payer's own account,
// however many passes and replicas see the same transfer.
func TestADepositFundsTheSpendableLedgerOnceAsCash(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	const org, subject = "acme", "acme/alice"
	seedOrg(t, org)
	l := inject(t)

	for pass := 1; pass <= 4; pass++ {
		if _, err := (intentStore{}).Credit(context.Background(), deposit(org, subject)); err != nil {
			t.Fatalf("pass %d: %v", pass, err)
		}
	}
	if len(l.posted) != 1 {
		t.Fatalf("the ledger took %d credits for one transfer, want 1", len(l.posted))
	}
	got := l.posted[0]
	if got.Org != org || got.Subject != subject || got.AmountCents != 2550 || got.Currency != "usd" {
		t.Errorf("funded %+v, want org %q subject %q 2550 usd", got, org, subject)
	}
	if !got.Cash {
		t.Errorf("a customer's own transfer must fund cash, not minted credit")
	}
	if got.IdempotencyKey != depositTag+":ethereum:0xfeedface:4" || got.Tag != depositTag {
		t.Errorf("key %q tag %q: the key must be the on-chain event so every path credits it once", got.IdempotencyKey, got.Tag)
	}
	if int64(balance(t, org, subject).Balance) != 2550 {
		t.Errorf("commerce's own row is still the record of the deposit")
	}
}

// A ledger that refuses must stop the pass before anything says the deposit was
// handled: no local row, so the next pass tries again rather than skipping money.
func TestALedgerFailureLeavesTheDepositToTheNextPass(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	const org, subject = "acme", "acme/alice"
	seedOrg(t, org)
	l := inject(t)
	l.fail = errors.New("ledger down")

	if _, err := (intentStore{}).Credit(context.Background(), deposit(org, subject)); err == nil {
		t.Fatal("a ledger failure must fail the credit")
	}
	if got := int64(balance(t, org, subject).Balance); got != 0 {
		t.Fatalf("a row was written (%d cents) for money the ledger never took: the next pass would skip it", got)
	}

	l.fail = nil
	written, err := (intentStore{}).Credit(context.Background(), deposit(org, subject))
	if err != nil || !written || len(l.posted) != 1 {
		t.Fatalf("retry: written=%v err=%v posted=%d, want true/nil/1", written, err, len(l.posted))
	}
}

// A deposit recorded before the ledger credit existed is money received and never
// made spendable. Seeing it again funds it.
func TestAnEarlierRowIsFundedWhenSeenAgain(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	const org, subject = "acme", "acme/alice"
	seedOrg(t, org)

	if _, err := (intentStore{}).Credit(context.Background(), deposit(org, subject)); err != nil {
		t.Fatalf("standalone credit: %v", err)
	}
	l := inject(t)
	written, err := (intentStore{}).Credit(context.Background(), deposit(org, subject))
	if err != nil {
		t.Fatal(err)
	}
	if written {
		t.Errorf("the local row already existed; nothing new was written there")
	}
	if len(l.posted) != 1 || !l.posted[0].Cash {
		t.Fatalf("the earlier deposit was not funded: %+v", l.posted)
	}
}

// A replay whose amount was recomputed differently replays the credit already
// made, so the ledger never refuses the key and stalls the asset.
func TestAReplayUsesTheAmountAlreadyRecorded(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	const org, subject = "acme", "acme/alice"
	seedOrg(t, org)
	l := inject(t)
	if _, err := (intentStore{}).Credit(context.Background(), deposit(org, subject)); err != nil {
		t.Fatal(err)
	}
	again := deposit(org, subject)
	again.AmountCents = 2400 // a fee changed across a restart
	l.seen = map[string]bool{}
	if _, err := (intentStore{}).Credit(context.Background(), again); err != nil {
		t.Fatal(err)
	}
	if got := l.posted[len(l.posted)-1].AmountCents; got != 2550 {
		t.Fatalf("the replay asked the ledger for %d cents, want the recorded 2550", got)
	}
}
