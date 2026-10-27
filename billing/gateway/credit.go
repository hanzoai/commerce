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
	"strconv"
	"strings"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/mintauth"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/remittance"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/providers/circle"
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

// ── the hold ─────────────────────────────────────────────────────────────────

// holdID is a payer's one hold row, per ledger and books.
func holdID(r *remittance.Remittance) string {
	sum := sha256.Sum256([]byte("remittance-hold\x00" + ledgerOrg(r) + "\x00" + r.Payer + "\x00" + strconv.FormatBool(r.Test)))
	return "hold_" + hex.EncodeToString(sum[:16])
}

// hold applies fn to the payer's hold row, writing only over the row read.
func hold(r *remittance.Remittance, fn func(*remittance.Hold) bool) (*remittance.Hold, error) {
	db := systemDB()
	id := holdID(r)
	for range mixin.Swaps {
		h := remittance.NewHold(db)
		switch err := h.Get(db.NewKey(h.Kind(), id, 0, nil)); {
		case errors.Is(err, datastore.ErrNoSuchEntity):
			h = remittance.NewHold(db)
			h.SetId(id)
			h.Org, h.Payer, h.Test = ledgerOrg(r), r.Payer, r.Test
		case err != nil:
			return nil, err
		}
		if h.Items == nil {
			h.Items = map[string]int64{}
		}
		if !fn(h) {
			return h, nil
		}
		ok, err := h.Swap()
		if err != nil {
			return nil, err
		}
		if ok {
			return h, nil
		}
	}
	return nil, fmt.Errorf("gateway: hold %s changed under %d reads in a row", id, mixin.Swaps)
}

// reserve puts the remittance in its payer's hold if the hold stays within
// MaxCents, and reports whether it is there. Reserving twice holds it once.
func (s *Service) reserve(r *remittance.Remittance) (bool, error) {
	held := false
	_, err := hold(r, func(h *remittance.Hold) bool {
		// Decided afresh on every read: a swap that lost is retried against
		// the row as it now stands, and an answer left over from the losing
		// read would reserve room that is no longer there.
		held = false
		if _, ok := h.Items[r.Id()]; ok {
			held = true
			return false
		}
		if h.Total()+r.AmountCents > MaxCents {
			return false
		}
		h.Items[r.Id()] = r.AmountCents
		held = true
		return true
	})
	return held, err
}

// release takes the remittance out of its payer's hold. Releasing what is not
// held is a no-op, so it is safe to call on every path that ends a hold.
func (s *Service) release(r *remittance.Remittance) error {
	_, err := hold(r, func(h *remittance.Hold) bool {
		if _, ok := h.Items[r.Id()]; !ok {
			return false
		}
		delete(h.Items, r.Id())
		return true
	})
	if err != nil {
		slog.Error("gateway: hold not released; retried on the next pass", "remittance", r.Id(), "err", err)
	}
	return err
}

// ── the credit ───────────────────────────────────────────────────────────────

// credit puts the remittance's value in the payer's account, once.
//
// A payment Gateway has confirmed onchain is credited outright. One Gateway
// has only locked is credited if it fits in the payer's hold; otherwise it is
// marked Deferred and credited when Gateway confirms it.
//
// It never returns an error: the payment itself is done — Gateway has the
// money — and a step that failed is retried by the reconciler from exactly the
// state it stopped in. The caller is told the truth by the remittance.
func (s *Service) credit(ctx context.Context, r *remittance.Remittance) *remittance.Remittance {
	if r.Landed || r.Reversed {
		return r
	}
	if !r.Credited {
		onchain := r.State == circle.Confirmed || r.State == circle.Completed
		if !onchain {
			held, err := s.reserve(r)
			if err != nil {
				slog.Error("gateway: hold not read; credit retried on the next pass", "remittance", r.Id(), "err", err)
				return r
			}
			if !held {
				out, err := update(r.Id(), func(x *remittance.Remittance) bool {
					if x.Deferred || x.Credited {
						return false
					}
					x.Deferred = true
					return true
				})
				if err != nil {
					return r
				}
				return out
			}
		}
		// OWED from here: written before the ledger is asked, so a payment
		// Gateway fails later is clawed back whatever happens next.
		out, err := update(r.Id(), func(x *remittance.Remittance) bool {
			if x.Credited {
				return false
			}
			x.Credited, x.Deferred = true, false
			return true
		})
		if err != nil {
			slog.Error("gateway: credit not marked; retried on the next pass", "remittance", r.Id(), "err", err)
			return r
		}
		r = out
		if r.Landed || r.Reversed {
			return r
		}
	}
	return s.land(ctx, r)
}

// land writes the owed credit to the ledger and the receipt, then marks it
// landed. Each write is keyed on the remittance, so landing twice lands once.
func (s *Service) land(ctx context.Context, r *remittance.Remittance) *remittance.Remittance {
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
		flipped = false
		if x.Landed {
			return false
		}
		x.Landed, flipped = true, true
		return true
	})
	if err != nil {
		slog.Error("RECONCILE: gateway: credited, flag not recorded; the next landing is a keyed replay", "remittance", r.Id(), "err", err)
		return r
	}
	if flipped {
		slog.Info("gateway: credited", "cents", r.AmountCents, "org", r.Org, "payer", r.Payer, "remittance", r.Id(), "payment", r.Payment)
	}
	return out
}

// reverse takes back an owed credit whose payment Gateway failed, once.
//
// It LANDS the credit first. The credit was owed from the moment Credited was
// written, and whether the ledger took it before the process stopped is
// exactly what may not be known; landing is a keyed replay when it did and the
// credit when it did not, so the clawback after it always takes back a credit
// that exists. Embedded, the clawback is the host ledger's (cash first, and it
// may leave the account below zero, where the spend gate refuses); standalone
// it is a withdrawal receipt beside the deposit one.
func (s *Service) reverse(ctx context.Context, r *remittance.Remittance) *remittance.Remittance {
	if !r.Credited || r.Reversed {
		return r
	}
	if r = s.land(ctx, r); !r.Landed {
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
		flipped = false
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
