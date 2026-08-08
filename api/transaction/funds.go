package transaction

import (
	"hash/fnv"
	"strconv"
	"sync"
)

// fundsLockStripes bounds the memory of the per-source funds lock to a fixed set
// of mutexes (no unbounded growth as sources come and go). Same key → same stripe
// → serialized; a rare hash collision only briefly serializes two unrelated
// sources, which is harmless.
const fundsLockStripes = 256

var fundsLockMu [fundsLockStripes]sync.Mutex

// lockFunds serializes the read-check-write on ONE source's funds, returning the
// unlock func.
//
// WHAT IT CLOSES. Both money paths here read a balance, decide against it, and
// then write:
//
//	datas := GetTransactionsByCurrency(...)          // read balance and holds
//	if data.Balance-data.Holds < trans.Amount { ... } // decide
//	return trans.Create()                             // write
//
// Two concurrent holds for 100 against a balance of 100 both read
// Balance-Holds=100, both pass, and both create — 200 held against 100. The
// invariant is "a SUM must not exceed a balance", which a deterministic key
// cannot express (each hold is legitimately its own row), so the check and the
// write have to be one step.
//
// WHY A LOCK AND NOT A TRANSACTION. datastore.RunInTransaction, which both paths
// already appeared to be inside, is a stub — "For now, just run the function
// directly" — so it opened nothing and gave no isolation. And the real one
// underneath cannot be dropped in here: db.SQLiteDB.RunInTransaction takes
// writeMu, SQLiteDB.Put takes the SAME writeMu, and Go mutexes are not
// reentrant — so wrapping trans.Create() in a real transaction deadlocks the
// process. Making that reentrant means threading the open transaction through
// the model layer's context at every nesting site, which is a refactor whose
// failure mode is a hung commerce.
//
// This is the same instrument billing/engine.lockPeriod already uses against the
// same class of bug, for the same stated reason: commerce is single-writer per
// tenant (ReadWriteOnce PVC, Recreate), so a process lock fully serializes the
// real concurrent callers. It is honestly bounded — a second writer process
// would race it — and that bound is the deployment's, not this function's.
func lockFunds(sourceKind, sourceID, currency string, test bool) func() {
	mu := &fundsLockMu[stripeOf(sourceKind, sourceID, currency, test)]
	mu.Lock()
	return mu.Unlock
}

// fundsKey names the one balance a caller is about to read, decide against, and
// write. It is the correctness boundary: two callers share a lock exactly when
// they share a key.
//
// Test money is a different ledger from live money, and one currency is a
// different balance from another, so both are in the key — a check against a USD
// balance says nothing about EUR.
func fundsKey(sourceKind, sourceID, currency string, test bool) string {
	return sourceKind + "|" + sourceID + "|" + currency + "|" + strconv.FormatBool(test)
}

// stripeOf maps a key onto the fixed set of mutexes. This is deliberately LOSSY:
// two unrelated keys can share a stripe, which only means they briefly serialize
// against each other. That is a throughput detail, never a correctness one — the
// guarantee runs the other way, that one key always maps to one stripe.
func stripeOf(sourceKind, sourceID, currency string, test bool) uintptr {
	h := fnv.New32a()
	_, _ = h.Write([]byte(fundsKey(sourceKind, sourceID, currency, test)))
	return uintptr(h.Sum32() % fundsLockStripes)
}
