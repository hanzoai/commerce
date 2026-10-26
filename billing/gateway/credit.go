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
// (api/billing/webhooks.go applySettlementEvent): the host's ledger FIRST,
// when commerce runs embedded and one is injected, because that is the balance
// the spend gate reads; then commerce's own receipt in the payer's org, which
// standalone IS the balance. Both are keyed on the remittance, so a credit
// attempted any number of times, from the request or the reconciler, on any
// number of replicas, lands once.

// tag classifies the receipt as real money: billing/bucket counts anything not
// a grant tag as Prepaid, which is what a customer's own USDC is.
const tag = "topup"

// ledgerKey is the idempotency key the host ledger dedups the credit on.
func ledgerKey(r *remittance.Remittance) string { return "remittance:" + r.Id() }

// ledgerOrg is the org as the ledger keys it (api/billing.ledgerOrg).
func ledgerOrg(r *remittance.Remittance) string { return strings.ToLower(strings.TrimSpace(r.Org)) }

// receipt is the deterministic key of one of the remittance's two possible
// receipts in the payer's org: the credit, and its reversal.
func receipt(db *datastore.Datastore, r *remittance.Remittance, kind string) datastore.Key {
	sum := sha256.Sum256([]byte("remittance\x00" + kind + "\x00" + r.Id()))
	root := db.NewKey("synckey", "", 1, nil)
	return db.NewKey("transaction", "rem_"+hex.EncodeToString(sum[:16]), 0, root)
}

func orgDB(org string) *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(context.Background(), org))
}

// credit puts the remittance's value in the payer's account, once, and marks
// it credited. A write that fails is logged and left for the reconciler: the
// remittance stays accepted and uncredited, which is exactly the state it
// retries from. It never returns an error, because the payment itself is
// done — Gateway has the money — and the caller is told the truth by the
// remittance's Credited flag.
func (s *Service) credit(ctx context.Context, r *remittance.Remittance) *remittance.Remittance {
	if r.Credited {
		return r
	}
	if led := creditledger.Get(); led != nil {
		if _, _, err := led.Credit(ctx, creditledger.CreditInput{
			Org:            ledgerOrg(r),
			Subject:        r.Payer,
			Currency:       "usd",
			Reason:         "circle gateway payment " + r.Payment,
			Tag:            tag,
			IdempotencyKey: ledgerKey(r),
			AmountCents:    r.AmountCents,
			Test:           r.Test,
			Cash:           true, // the payer's own USDC, locked by Gateway
		}); err != nil {
			slog.Error("RECONCILE: gateway: accepted by Circle and NOT credited; the reconciler retries, keyed on the remittance", "remittance", r.Id(), "payment", r.Payment, "cents", r.AmountCents, "org", r.Org, "payer", r.Payer, "err", err)
			return r
		}
	}
	if err := s.write(r, "credit", transaction.Deposit); err != nil {
		slog.Error("RECONCILE: gateway: receipt not written; the reconciler retries", "remittance", r.Id(), "org", r.Org, "err", err)
		return r
	}
	flipped := false
	out, err := update(r.Id(), func(x *remittance.Remittance) bool {
		if x.Credited {
			return false
		}
		x.Credited, flipped = true, true
		return true
	})
	if err != nil {
		slog.Error("RECONCILE: gateway: credited, flag not recorded; the next credit is a keyed replay", "remittance", r.Id(), "err", err)
		return r
	}
	if flipped {
		slog.Info("gateway: credited", "cents", r.AmountCents, "org", r.Org, "payer", r.Payer, "remittance", r.Id(), "payment", r.Payment)
	}
	return out
}

// reverse takes back a credit whose payment Gateway failed, once. Embedded, it
// is the host ledger's Clawback, which takes cash first and may leave the
// account below zero, where the spend gate refuses; standalone, it is a
// withdrawal receipt beside the deposit one.
func (s *Service) reverse(ctx context.Context, r *remittance.Remittance) *remittance.Remittance {
	if !r.Credited || r.Reversed {
		return r
	}
	if led := creditledger.Get(); led != nil {
		if _, err := led.Clawback(ctx, creditledger.ClawbackInput{
			Org:         ledgerOrg(r),
			Subject:     r.Payer,
			Payment:     ledgerKey(r),
			Ref:         "remittance-failed:" + r.Id(),
			AmountCents: r.AmountCents,
			Reason:      "circle gateway payment " + r.Payment + " failed",
			Test:        r.Test,
		}); err != nil {
			slog.Error("RECONCILE: gateway: FAILED at Circle and the credit was NOT reversed; the reconciler retries", "remittance", r.Id(), "cents", r.AmountCents, "org", r.Org, "payer", r.Payer, "err", err)
			return r
		}
	}
	if err := s.write(r, "reversal", transaction.Withdraw); err != nil {
		slog.Error("RECONCILE: gateway: reversal receipt not written; the reconciler retries", "remittance", r.Id(), "org", r.Org, "err", err)
		return r
	}
	flipped := false
	out, err := update(r.Id(), func(x *remittance.Remittance) bool {
		if x.Reversed {
			return false
		}
		x.Reversed, flipped = true, true
		return true
	})
	if err != nil {
		slog.Error("RECONCILE: gateway: reversed, flag not recorded", "remittance", r.Id(), "err", err)
		return r
	}
	if flipped {
		slog.Warn("gateway: reversed a credit Circle failed", "cents", r.AmountCents, "org", r.Org, "payer", r.Payer, "remittance", r.Id(), "payment", r.Payment)
	}
	return out
}

// write puts one receipt for r in the payer's org, unless it is already there.
// Both backends upsert on (id, kind, namespace), so two writers racing write
// one row.
func (s *Service) write(r *remittance.Remittance, kind string, typ transaction.Type) error {
	db := orgDB(r.Org)
	key := receipt(db, r, kind)
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
	t.Type = typ
	if typ == transaction.Deposit {
		t.DestinationId, t.DestinationKind = r.Payer, transaction.IAMUserKind
	} else {
		t.SourceId, t.SourceKind = r.Payer, transaction.IAMUserKind
	}
	t.Currency = currency.Type("usd")
	t.Amount = currency.Cents(r.AmountCents)
	t.Tags = tag
	t.Test = r.Test
	t.Notes = fmt.Sprintf("Circle Gateway %s: %s USDC base units from %s on %s (payment %s)", kind, r.Value, r.From, r.Network, r.Payment)
	t.Metadata = types.Map{
		"source":     "circle-gateway",
		"remittance": r.Id(),
		"payment":    r.Payment,
		"network":    r.Network,
		"from":       r.From,
		"nonce":      r.Nonce,
	}
	// The authority for this money is Gateway's acceptance of a payment the
	// payer signed — an external, settled fact like a card capture — and it is
	// stated here, at the ledger sink, explicitly.
	t.SetContext(mintauth.WithAuthorized(t.Context()))
	return t.Create()
}
