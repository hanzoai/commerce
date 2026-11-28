package creditledger

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ErrDebitConflict is returned when a Debit reuses a Ref with a different amount or address.
var ErrDebitConflict = errors.New("creditledger: debit ref reused with conflicting parameters")

type bookAddr struct {
	org      string
	subject  string
	currency string
	test     bool
}

type creditLot struct {
	amount    int64
	remaining int64
	cash      bool
	expiresAt *time.Time
}

type bookState struct {
	lots []*creditLot
	debt int64
}

type creditReceipt struct {
	txID     string
	currency string
	addr     bookAddr
}

type debitRecord struct {
	txID       string
	addr       bookAddr
	amount     int64
	refunded   bool
	refundTxID string
}

type clawbackRecord struct {
	txID        string
	addr        bookAddr
	amount      int64
	cashTaken   int64
	creditTaken int64
	debtAdded   int64
}

type doubleEntryPosting struct {
	txID    string
	account string
	amount  int64
}

// MemCreditLedger is an in-memory, concurrency-safe double-entry CreditLedger
// enforcing idempotent credits/debits/refunds/clawbacks/restores and atomic
// insufficient-balance refusal.
type MemCreditLedger struct {
	mu        sync.RWMutex
	nowFn     func() time.Time
	seq       atomic.Int64
	books     map[bookAddr]*bookState
	credits   map[string]creditReceipt  // "org|test|idemKey" -> receipt
	debits    map[string]*debitRecord   // "org|test|ref" -> debit
	clawbacks map[string]clawbackRecord // "org|test|ref" -> clawback
	restores  map[string]string         // "org|test|ref" -> restore txID
	postings  []doubleEntryPosting
}

var _ CreditLedger = (*MemCreditLedger)(nil)

// NewMemCreditLedger returns an initialized in-memory CreditLedger.
func NewMemCreditLedger() *MemCreditLedger {
	return &MemCreditLedger{
		nowFn:     time.Now,
		books:     make(map[bookAddr]*bookState),
		credits:   make(map[string]creditReceipt),
		debits:    make(map[string]*debitRecord),
		clawbacks: make(map[string]clawbackRecord),
		restores:  make(map[string]string),
	}
}

func normalizeAddr(org, subject, currency string, test bool) bookAddr {
	o := strings.ToLower(strings.TrimSpace(org))
	s := strings.ToLower(strings.TrimSpace(subject))
	if s == "" {
		s = o
	}
	c := strings.ToLower(strings.TrimSpace(currency))
	if c == "" {
		c = "usd"
	}
	return bookAddr{
		org:      o,
		subject:  s,
		currency: c,
		test:     test,
	}
}

func ledgerScope(org string, test bool, key string) string {
	return fmt.Sprintf("%s|%t|%s", strings.ToLower(strings.TrimSpace(org)), test, key)
}

func (a bookAddr) bookAccount() string {
	return fmt.Sprintf("book:%s:%s:%s:%t", a.org, a.subject, a.currency, a.test)
}

func (a bookAddr) houseAccount() string {
	return fmt.Sprintf("house:%s:%s:%t", a.org, a.currency, a.test)
}

func (m *MemCreditLedger) now() time.Time {
	if m.nowFn != nil {
		return m.nowFn()
	}
	return time.Now()
}

func (m *MemCreditLedger) nextID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, m.seq.Add(1))
}

func (m *MemCreditLedger) bookLocked(addr bookAddr) *bookState {
	b, ok := m.books[addr]
	if !ok {
		b = &bookState{}
		m.books[addr] = b
	}
	return b
}

func (m *MemCreditLedger) recordBalancedPostingsLocked(txID string, addr bookAddr, delta int64) {
	m.postings = append(m.postings,
		doubleEntryPosting{txID: txID, account: addr.bookAccount(), amount: delta},
		doubleEntryPosting{txID: txID, account: addr.houseAccount(), amount: -delta},
	)
}

func (m *MemCreditLedger) availableLocked(addr bookAddr, now time.Time) int64 {
	b, ok := m.books[addr]
	if !ok {
		return 0
	}
	var avail int64
	for _, lot := range b.lots {
		if lot.remaining <= 0 {
			continue
		}
		if lot.expiresAt != nil && !lot.expiresAt.After(now) {
			continue
		}
		avail += lot.remaining
	}
	return avail - b.debt
}

func (m *MemCreditLedger) addLotLocked(b *bookState, amount int64, cash bool, exp *time.Time) {
	rem := amount
	if b.debt > 0 {
		repay := min(rem, b.debt)
		b.debt -= repay
		rem -= repay
	}
	if rem > 0 {
		var expCopy *time.Time
		if exp != nil {
			t := *exp
			expCopy = &t
		}
		b.lots = append(b.lots, &creditLot{
			amount:    rem,
			remaining: rem,
			cash:      cash,
			expiresAt: expCopy,
		})
	}
}

func consumeLotsLocked(b *bookState, need int64, now time.Time, cashOnly bool) int64 {
	var taken int64
	for _, lot := range b.lots {
		if taken >= need {
			break
		}
		if lot.remaining <= 0 || lot.cash != cashOnly {
			continue
		}
		if lot.expiresAt != nil && !lot.expiresAt.After(now) {
			continue
		}
		take := min(lot.remaining, need-taken)
		lot.remaining -= take
		taken += take
	}
	return taken
}

// Credit appends an idempotent double-entry credit grant to (Org, Subject, Currency, Test).
func (m *MemCreditLedger) Credit(_ context.Context, in CreditInput) (string, int64, error) {
	if in.AmountCents <= 0 {
		return "", 0, errors.New("creditledger: credit amount must be positive")
	}
	addr := normalizeAddr(in.Org, in.Subject, in.Currency, in.Test)
	if addr.org == "" {
		return "", 0, errors.New("creditledger: org is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	if in.IdempotencyKey != "" {
		scope := ledgerScope(addr.org, addr.test, in.IdempotencyKey)
		if existing, ok := m.credits[scope]; ok {
			return existing.txID, m.availableLocked(existing.addr, now), nil
		}
	}

	txID := m.nextID("cr")
	b := m.bookLocked(addr)
	m.addLotLocked(b, in.AmountCents, in.Cash, in.ExpiresAt)
	m.recordBalancedPostingsLocked(txID, addr, in.AmountCents)

	if in.IdempotencyKey != "" {
		scope := ledgerScope(addr.org, addr.test, in.IdempotencyKey)
		m.credits[scope] = creditReceipt{
			txID:     txID,
			currency: addr.currency,
			addr:     addr,
		}
	}

	return txID, m.availableLocked(addr, now), nil
}

// Balance returns the available balance in cents held at (org, subject, currency, test).
func (m *MemCreditLedger) Balance(_ context.Context, org, subject, currency string, test bool) (int64, error) {
	addr := normalizeAddr(org, subject, currency, test)

	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.availableLocked(addr, m.now()), nil
}

// Debit atomically checks available balance and deducts in.AmountCents once per Ref.
func (m *MemCreditLedger) Debit(_ context.Context, in DebitInput) (string, int64, error) {
	if in.AmountCents <= 0 {
		return "", 0, errors.New("creditledger: debit amount must be positive")
	}
	addr := normalizeAddr(in.Org, in.Subject, in.Currency, in.Test)
	if addr.org == "" || strings.TrimSpace(in.Ref) == "" {
		return "", 0, errors.New("creditledger: org and ref are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	scope := ledgerScope(addr.org, addr.test, "debit:"+in.Ref)
	if rec, ok := m.debits[scope]; ok && !rec.refunded {
		if rec.addr != addr || rec.amount != in.AmountCents {
			return "", m.availableLocked(addr, now), ErrDebitConflict
		}
		return rec.txID, m.availableLocked(addr, now), nil
	}

	avail := m.availableLocked(addr, now)
	if avail < in.AmountCents {
		return "", avail, ErrShort
	}

	b := m.bookLocked(addr)
	rem := in.AmountCents
	takenPromo := consumeLotsLocked(b, rem, now, false)
	rem -= takenPromo
	if rem > 0 {
		takenCash := consumeLotsLocked(b, rem, now, true)
		rem -= takenCash
	}
	if rem != 0 {
		return "", m.availableLocked(addr, now), ErrShort
	}

	txID := m.nextID("db")
	m.debits[scope] = &debitRecord{
		txID:   txID,
		addr:   addr,
		amount: in.AmountCents,
	}
	m.recordBalancedPostingsLocked(txID, addr, -in.AmountCents)

	return txID, m.availableLocked(addr, now), nil
}

// Refund idempotently restores the debit drawn under in.Ref at (Org, Subject, Test).
func (m *MemCreditLedger) Refund(_ context.Context, in RefundInput) (string, error) {
	org := strings.ToLower(strings.TrimSpace(in.Org))
	subj := strings.ToLower(strings.TrimSpace(in.Subject))
	if subj == "" {
		subj = org
	}
	if org == "" || strings.TrimSpace(in.Ref) == "" {
		return "", errors.New("creditledger: org and ref are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	scope := ledgerScope(org, in.Test, "debit:"+in.Ref)
	rec, ok := m.debits[scope]
	if !ok {
		return "", nil
	}
	if rec.addr.org != org || rec.addr.subject != subj || rec.addr.test != in.Test {
		return "", nil
	}
	if rec.refunded {
		return rec.refundTxID, nil
	}

	txID := m.nextID("rf")
	rec.refunded = true
	rec.refundTxID = txID

	b := m.bookLocked(rec.addr)
	m.addLotLocked(b, rec.amount, false, nil)
	m.recordBalancedPostingsLocked(txID, rec.addr, rec.amount)

	return txID, nil
}

// Clawback removes funds returned to a payer under in.Payment once per in.Ref
// (cash first, then credit, allowing negative balance if overdrawn).
func (m *MemCreditLedger) Clawback(_ context.Context, in ClawbackInput) (string, error) {
	if in.AmountCents <= 0 {
		return "", errors.New("creditledger: clawback amount must be positive")
	}
	org := strings.ToLower(strings.TrimSpace(in.Org))
	if org == "" || strings.TrimSpace(in.Ref) == "" {
		return "", errors.New("creditledger: org and ref are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	cScope := ledgerScope(org, in.Test, in.Ref)
	if existing, ok := m.clawbacks[cScope]; ok {
		return existing.txID, nil
	}

	cur := "usd"
	if in.Payment != "" {
		receipt, ok := m.credits[ledgerScope(org, in.Test, in.Payment)]
		if !ok {
			return "", fmt.Errorf("creditledger: payment %s not found in (%s, test=%t)", in.Payment, org, in.Test)
		}
		cur = receipt.currency
	}

	addr := normalizeAddr(org, in.Subject, cur, in.Test)
	now := m.now()
	b := m.bookLocked(addr)

	cashTaken := consumeLotsLocked(b, in.AmountCents, now, true)
	rem := in.AmountCents - cashTaken
	creditTaken := consumeLotsLocked(b, rem, now, false)
	rem -= creditTaken
	var debtAdded int64
	if rem > 0 {
		b.debt += rem
		debtAdded = rem
	}

	txID := m.nextID("cb")
	m.clawbacks[cScope] = clawbackRecord{
		txID:        txID,
		addr:        addr,
		amount:      in.AmountCents,
		cashTaken:   cashTaken,
		creditTaken: creditTaken,
		debtAdded:   debtAdded,
	}
	m.recordBalancedPostingsLocked(txID, addr, -in.AmountCents)

	return txID, nil
}

// Restore reverses a prior Clawback posted under in.Ref once per in.Ref.
func (m *MemCreditLedger) Restore(_ context.Context, in ClawbackInput) (string, error) {
	org := strings.ToLower(strings.TrimSpace(in.Org))
	if org == "" || strings.TrimSpace(in.Ref) == "" {
		return "", errors.New("creditledger: org and ref are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	rScope := ledgerScope(org, in.Test, in.Ref)
	if txID, ok := m.restores[rScope]; ok {
		return txID, nil
	}

	cb, ok := m.clawbacks[rScope]
	if !ok {
		return "", nil
	}

	txID := m.nextID("rs")
	b := m.bookLocked(cb.addr)
	if cb.debtAdded > 0 {
		if b.debt >= cb.debtAdded {
			b.debt -= cb.debtAdded
		} else {
			excess := cb.debtAdded - b.debt
			b.debt = 0
			m.addLotLocked(b, excess, false, nil)
		}
	}
	if cb.cashTaken > 0 {
		m.addLotLocked(b, cb.cashTaken, true, nil)
	}
	if cb.creditTaken > 0 {
		m.addLotLocked(b, cb.creditTaken, false, nil)
	}

	m.restores[rScope] = txID
	m.recordBalancedPostingsLocked(txID, cb.addr, cb.amount)

	return txID, nil
}

// LedgerBalanceSum returns the net sum of all double-entry postings across all
// books and house accounts (invariant: always zero).
func (m *MemCreditLedger) LedgerBalanceSum() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var total int64
	for _, p := range m.postings {
		total += p.amount
	}
	return total
}
