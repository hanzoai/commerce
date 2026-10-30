// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/mintauth"
	"github.com/hanzoai/commerce/models/remittance"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/nscontext"
)

// The money is written the way every settled top-up is written
// (api/billing/webhooks.go applySettlementEvent, billing/depositledger Credit):
// the host's ledger FIRST, when commerce runs embedded and one is injected,
// because that is the balance the spend gate reads; then commerce's own
// receipt in the payer's org, which standalone IS the balance. Both are keyed
// on the remittance, so a credit attempted any number of times, from the
// request or the reconciler, on any number of replicas, lands once.

// tag classifies the receipt as real money: billing/bucket counts anything not
// a grant tag as Prepaid, which is what a customer's own USDC is.
const tag = "topup"

// ledgerKey is the idempotency key the host ledger dedups the credit on.
func ledgerKey(r *remittance.Remittance) string { return "remittance:" + r.Id() }

// ledgerOrg is the org as the ledger keys it (api/billing.ledgerOrg).
func ledgerOrg(r *remittance.Remittance) string { return strings.ToLower(strings.TrimSpace(r.Org)) }

// receipt is the deterministic key of the remittance's receipt in the payer's
// org.
func receipt(db *datastore.Datastore, r *remittance.Remittance) datastore.Key {
	sum := sha256.Sum256([]byte("remittance\x00credit\x00" + r.Id()))
	root := db.NewKey("synckey", "", 1, nil)
	return db.NewKey("transaction", "rem_"+hex.EncodeToString(sum[:16]), 0, root)
}

func orgDB(org string) *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(context.Background(), org))
}

// land writes the owed credit to the ledger and the receipt, then marks the
// remittance succeeded. Each write is keyed on the remittance, so landing twice
// lands once. It never returns an error: the money is the treasury's, and a
// step that failed is retried by the reconciler from exactly the state it
// stopped in.
func (s *Service) land(ctx context.Context, r *remittance.Remittance) *remittance.Remittance {
	if !r.Credited || r.Landed {
		return r
	}
	if led := creditledger.Get(); led != nil {
		if _, _, err := led.Credit(ctx, creditledger.CreditInput{
			Org:            ledgerOrg(r),
			Subject:        r.Payer,
			Currency:       "usd",
			Reason:         "usdc authorization " + r.TxHash,
			Tag:            tag,
			IdempotencyKey: ledgerKey(r),
			AmountCents:    r.AmountCents,
			Test:           r.Test,
			Cash:           true, // the payer's own USDC, in the treasury
		}); err != nil {
			slog.Error("RECONCILE: gateway: paid on chain and NOT credited; the reconciler retries, keyed on the remittance", "remittance", r.Id(), "tx", r.TxHash, "cents", r.AmountCents, "org", r.Org, "payer", r.Payer, "err", err)
			return r
		}
	}
	if err := s.write(r); err != nil {
		slog.Error("RECONCILE: gateway: receipt not written; the reconciler retries", "remittance", r.Id(), "org", r.Org, "err", err)
		return r
	}
	flipped := false
	out, err := update(r.Id(), func(x *remittance.Remittance) bool {
		flipped = false
		if x.Landed {
			return false
		}
		x.Landed, flipped = true, true
		x.Status, x.Due, x.SettledAt = remittance.Succeeded, never, s.now()
		return true
	})
	if err != nil {
		slog.Error("RECONCILE: gateway: credited, flag not recorded; the next landing is a keyed replay", "remittance", r.Id(), "err", err)
		return r
	}
	if flipped {
		slog.Info("audit: gateway credited a USDC authorization", "cents", r.AmountCents, "org", r.Org, "payer", r.Payer, "remittance", r.Id(), "tx", r.TxHash, "network", r.Network)
	}
	return out
}

// write puts the receipt for r in the payer's org, unless it is already there.
// Both backends upsert on (id, kind, namespace), so two writers racing write
// one row.
func (s *Service) write(r *remittance.Remittance) error {
	db := orgDB(r.Org)
	key := receipt(db, r)
	existing := transaction.New(db)
	switch err := existing.Get(key); {
	case err == nil:
		return nil
	case !errors.Is(err, datastore.ErrNoSuchEntity):
		return fmt.Errorf("check receipt: %w", err)
	}
	t := transaction.New(db)
	if err := t.SetKey(key); err != nil {
		return fmt.Errorf("set receipt key: %w", err)
	}
	t.Type = transaction.Deposit
	t.DestinationId, t.DestinationKind = r.Payer, transaction.IAMUserKind
	t.Currency = currency.Type("usd")
	t.Amount = currency.Cents(r.AmountCents)
	t.Tags = tag
	t.Test = r.Test
	t.Notes = fmt.Sprintf("USDC authorization: %s base units from %s on %s (transaction %s)", r.Value, r.From, r.Network, r.TxHash)
	t.Metadata = types.Map{
		"source":     "usdc-authorization",
		"remittance": r.Id(),
		"network":    r.Network,
		"from":       r.From,
		"nonce":      r.Nonce,
		"tx":         r.TxHash,
	}
	// The authority for this money is the chain: a confirmed transfer the
	// payer signed, into the treasury — an external, settled fact like a card
	// capture — and it is stated here, at the ledger sink, explicitly.
	t.SetContext(mintauth.WithAuthorized(t.Context()))
	return t.Create()
}
