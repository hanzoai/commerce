package depositledger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/mintauth"
	"github.com/hanzoai/commerce/models/cryptopaymentintent"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/nscontext"
)

// depositTag is the ledger tag every credited crypto deposit carries. It must
// classify as billing/bucket.Prepaid — REAL MONEY — because that is what it is:
// the customer bought balance with dollars. Landing it in the non-cash Credit
// bucket would be a quiet theft of capability, since credits may not be spent on
// GPUs. TestDepositTag_IsRealMoney pins this.
const depositTag = "crypto-deposit"

// intentStore is the production depositwatch.Store: per-org deposit intents on
// one side, the commerce transaction ledger on the other.
type intentStore struct{}

var _ depositwatch.Store = intentStore{}

// orgDB scopes the CENTRAL store to an org by namespace — the same store and
// scoping the card top-up, the HUSD projection and every balance read use. NOT
// NewNamespaced, which would split these credits into per-org SQLite files the
// balance gate never reads (see the note on api/billing.Deposit).
func orgDB(org string) *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(context.Background(), org))
}

// Watched enumerates every minted deposit address for (chain, token), across
// every org.
//
// It filters on the ASSET and deliberately not on the intent's status. An
// address is a real custody destination from the moment it is minted: money sent
// to an expired, failed or already-settled intent is still the customer's money,
// and refusing to look at it would recreate — in a narrower form — the exact
// hole this package closes. Expiry governs whether we hand the address out
// again (api/billing.CreateCryptoDeposit), never whether we honour what arrives
// at it.
//
// Organizations are global (DefaultNamespace), so ONE enumeration reaches every
// tenant.
func (intentStore) Watched(_ context.Context, chain, token string) ([]depositwatch.Watched, error) {
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(datastore.New(context.Background())).GetAll(&orgs); err != nil {
		return nil, fmt.Errorf("depositledger: list organizations: %w", err)
	}

	var out []depositwatch.Watched
	for _, o := range orgs {
		name := strings.TrimSpace(o.Name)
		if name == "" {
			continue
		}
		db := orgDB(name)
		intents := make([]*cryptopaymentintent.CryptoPaymentIntent, 0)
		keys, err := cryptopaymentintent.Query(db).
			Filter("Chain=", chain).
			Filter("Token=", token).
			GetAll(&intents)
		if err != nil {
			// One org's unreadable intents must fail the pass, not be skipped:
			// skipping is indistinguishable from "that org has no deposits", and
			// the difference is somebody's money.
			return nil, fmt.Errorf("depositledger: list %s/%s deposit intents for org %s: %w", chain, token, name, err)
		}
		for i, in := range intents {
			if in.DepositAddress == "" || in.CustomerRef == "" {
				continue // never minted an address, or names no payer to credit
			}
			out = append(out, depositwatch.Watched{
				Org:      name,
				Test:     o.TestMode(),
				IntentID: db.EncodeKey(keys[i]),
				Subject:  in.CustomerRef,
				Address:  in.DepositAddress,
				Status:   in.Status,
				TxHash:   in.TxHash,
				Block:    uint64(in.BlockNumber),
			})
		}
	}
	return out, nil
}

// Sight records a deposit that has been seen but is not yet deep enough to
// credit. It moves display state only. Idempotent: re-sighting the same
// transaction just deepens the confirmation count, and an intent that has moved
// on is left alone.
//
// It NEVER returns an error, and that is a deliberate split rather than
// swallowed error handling: this write moves no money, so failing the pass on it
// would stop every OTHER customer's deposit from being credited behind one
// customer's unreadable intent row. The failure is logged, and the next pass
// retries it — the block is still inside the re-scan window.
func (intentStore) Sight(_ context.Context, s depositwatch.Sighting) error {
	db := orgDB(s.Org)
	in := cryptopaymentintent.New(db)
	if err := in.GetById(s.IntentID); err != nil {
		log.Warn("depositledger: cannot sight intent %s (%s): %v", s.IntentID, s.TxHash, err)
		return nil
	}
	switch {
	case in.Status == cryptopaymentintent.Pending:
		if err := in.MarkConfirming(s.TxHash, int64(s.Block)); err != nil {
			return err
		}
	case in.Status == cryptopaymentintent.Confirming && strings.EqualFold(in.TxHash, s.TxHash):
		// The same deposit, deeper. Fall through and update the count.
	default:
		// Settled, expired, or already confirming a DIFFERENT deposit. The intent
		// carries one narrative; the money is carried per-transfer by the ledger,
		// so nothing is lost by leaving it.
		return nil
	}
	in.Confirmations = s.Confirmations
	in.BlockNumber = int64(s.Block)
	if err := in.Update(); err != nil {
		log.Warn("depositledger: cannot record sighting on intent %s: %v", s.IntentID, err)
	}
	return nil
}

// Unsight returns an intent to pending after the deposit it was confirming left
// the canonical chain. Display-only, and non-fatal for the same reason Sight is.
func (intentStore) Unsight(_ context.Context, s depositwatch.Sighting) error {
	db := orgDB(s.Org)
	in := cryptopaymentintent.New(db)
	if err := in.GetById(s.IntentID); err != nil {
		log.Warn("depositledger: cannot un-sight intent %s: %v", s.IntentID, err)
		return nil
	}
	if in.Status != cryptopaymentintent.Confirming || !strings.EqualFold(in.TxHash, s.TxHash) {
		return nil // already moved on; there is nothing to undo
	}
	if err := in.ClearSighting(); err != nil {
		return nil // not confirming any more; nothing to undo
	}
	if err := in.Update(); err != nil {
		log.Warn("depositledger: cannot clear reorged sighting on intent %s: %v", s.IntentID, err)
	}
	log.Info("depositledger: intent %s returned to pending — %s left the canonical chain", s.IntentID, s.TxHash)
	return nil
}

// creditKey is the DETERMINISTIC storage key for a credited deposit: a pure
// function of the on-chain event (chain:txHash:logIndex).
//
// THIS IS WHERE EXACTLY-ONCE LIVES. Both storage backends upsert on
// (id, kind, namespace) — db/sqlite.go and db/postgres.go — so every attempt to
// credit the same transfer lands on the SAME ROW, whether it comes from a
// re-scan of the reorg window, a retry after a crash, or two replicas ticking at
// the same instant. The balance is a sum over rows, so one row is one credit.
//
// That is deliberately a property of the KEY and not of coordination: there is
// no lock, no lease and no leader election to get wrong, and adding replicas
// cannot change the answer.
func creditKey(db *datastore.Datastore, dedup string) datastore.Key {
	sum := sha256.Sum256([]byte("crypto-deposit\x00" + dedup))
	root := db.NewKey("synckey", "", 1, nil)
	return db.NewKey("transaction", "cdep_"+hex.EncodeToString(sum[:16]), 0, root)
}

// Credit writes the idempotent ledger row and THEN advances the intent.
//
// The order is load-bearing. If the intent advanced first and the ledger write
// then failed, a later pass could read a succeeded intent and conclude the work
// was done — money received, never credited. In this order the worst case is a
// pass that re-writes a row it already wrote, which the deterministic key makes
// a no-op. An already-written row therefore still falls through to the intent
// advance, so a partially-completed credit heals on the next pass.
func (intentStore) Credit(_ context.Context, c depositwatch.Credit) (bool, error) {
	if c.Org == "" || c.Subject == "" {
		return false, fmt.Errorf("depositledger: credit %s names no org/payer", c.DedupKey)
	}
	if c.AmountCents <= 0 {
		return false, fmt.Errorf("depositledger: credit %s is %d cents", c.DedupKey, c.AmountCents)
	}
	db := orgDB(c.Org)
	key := creditKey(db, c.DedupKey)

	written := false
	existing := transaction.New(db)
	switch err := existing.Get(key); {
	case err == nil:
		// Already credited by an earlier pass or another replica.
	case errors.Is(err, datastore.ErrNoSuchEntity):
		trans := transaction.New(db)
		if err := trans.SetKey(key); err != nil {
			return false, fmt.Errorf("depositledger: set credit key: %w", err)
		}
		trans.Type = transaction.Deposit
		trans.DestinationId = c.Subject
		trans.DestinationKind = transaction.IAMUserKind
		trans.Currency = currency.Type("usd")
		trans.Amount = currency.Cents(c.AmountCents)
		trans.Tags = depositTag
		trans.Test = c.Test
		trans.Notes = fmt.Sprintf("Crypto deposit: %s %s on %s (%s)", c.Units, strings.ToUpper(c.Token), c.Chain, c.TxHash)
		trans.Metadata = types.Map{
			"source": "crypto-deposit",
			"chain":  c.Chain,
			"token":  c.Token,
			"txHash": c.TxHash,
			// The event's position within its transaction, read per chain: a log
			// index on the EVM, a token-balance record on Solana. Named for the
			// concept and not for the EVM's spelling of it, because the ledger row
			// is the permanent record and a field called logIndex holding a Solana
			// account index is a lie a future reader cannot detect.
			"eventIndex":    c.EventIndex,
			"blockNumber":   c.Block,
			"confirmations": c.Confirmations,
			"units":         c.Units,
			"rate":          c.PegRate,
			"intentId":      c.IntentID,
		}
		// The value being mirrored was created by a customer's own on-chain
		// transfer into a custody address we minted — a settled, external fact,
		// exactly like a card settlement. The context is a background job and so
		// is ungated anyway; this is the explicit statement of authority the
		// ledger sink (mintauth.Enforce) checks.
		trans.SetContext(mintauth.WithAuthorized(trans.Context()))
		if err := trans.Create(); err != nil {
			return false, fmt.Errorf("depositledger: write credit %s: %w", c.DedupKey, err)
		}
		written = true
		log.Info("depositledger: credited %d cents to %s from %s", c.AmountCents, c.Subject, c.DedupKey)
	default:
		// Cannot tell a first credit from a repeat: refuse rather than risk
		// either double-crediting or losing the deposit.
		return false, fmt.Errorf("depositledger: check existing credit %s: %w", c.DedupKey, err)
	}

	// The money is safe from here. An intent that will not advance is a DISPLAY
	// defect, so it is logged and not returned: failing the pass would leave the
	// cursor parked and stop crediting everyone else behind one stale row.
	if err := advanceIntent(db, c); err != nil {
		log.Warn("depositledger: credit %s written, intent %s not advanced: %v", c.DedupKey, c.IntentID, err)
	}
	return written, nil
}

// advanceIntent moves the intent to succeeded now that its deposit is credited.
// It never reports a problem as fatal to the money: the ledger row is already
// written, so a stale intent is a display defect, and the error is returned only
// so it is visible in logs and retried on the next pass.
func advanceIntent(db *datastore.Datastore, c depositwatch.Credit) error {
	in := cryptopaymentintent.New(db)
	if err := in.GetById(c.IntentID); err != nil {
		return fmt.Errorf("depositledger: credit %s written, but intent %s could not be loaded: %w", c.DedupKey, c.IntentID, err)
	}
	switch in.Status {
	case cryptopaymentintent.Succeeded, cryptopaymentintent.Refunded:
		return nil // terminal; a repeat deposit is carried by its own ledger row
	case cryptopaymentintent.Confirming:
		if !strings.EqualFold(in.TxHash, c.TxHash) {
			return nil // confirming a different deposit; leave that narrative alone
		}
	default:
		if err := in.MarkConfirming(c.TxHash, int64(c.Block)); err != nil {
			return err
		}
	}
	// Record the policy actually applied, so the intent cannot disagree with the
	// depth it was judged by (an intent minted before a policy change carries the
	// old number).
	in.RequiredConfirmations = cryptopaymentintent.RequiredConfirmationsForChain(in.Chain)
	in.Confirmations = c.Confirmations
	in.BlockNumber = int64(c.Block)
	in.CryptoAmount = c.Units
	in.SettlementCurrency = "usd"
	if err := in.MarkSucceeded(c.AmountCents, c.PegRate); err != nil {
		return err
	}
	return in.Update()
}
