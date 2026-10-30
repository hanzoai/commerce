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
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hanzoai/commerce/billing/custody/evm"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/remittance"
	"github.com/hanzoai/commerce/util/nscontext"
)

// View is a remittance as its payer reads it. Credited means the money is in
// the payer's account.
type View struct {
	ID                    string `json:"id"`
	Status                string `json:"status"`
	Network               string `json:"network"`
	From                  string `json:"from"`
	Value                 string `json:"value"`
	AmountCents           int64  `json:"amountCents"`
	Credited              bool   `json:"credited"`
	Reason                string `json:"reason,omitempty"`
	TxHash                string `json:"txHash,omitempty"`
	Confirmations         int64  `json:"confirmations"`
	RequiredConfirmations int64  `json:"requiredConfirmations"`
	CreatedAt             string `json:"createdAt"`
	UpdatedAt             string `json:"updatedAt"`
}

func (s *Service) view(r *remittance.Remittance) View {
	v := View{
		ID: r.Id(), Status: r.Status, Network: r.Network, From: r.From, Value: r.Value,
		AmountCents: r.AmountCents, Credited: r.Landed, Reason: r.Reason, TxHash: r.TxHash,
		Confirmations: r.Confirmations,
		CreatedAt:     r.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: r.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if c, ok := lookup(s.cfg.Chains, r.Network); ok {
		v.RequiredConfirmations = c.Confirmations
	}
	if r.TxHash == "" && len(r.Sends) > 0 {
		v.TxHash = r.Sends[len(r.Sends)-1] // the relayer's transaction, not yet seen mined
	}
	return v
}

// ID is a payment's storage id: a pure function of its identity on the chain.
// The USDC contract refuses a second use of a (payer address, nonce), so these
// three name one payment and nothing else names it.
func ID(network, from, nonce string) string {
	sum := sha256.Sum256([]byte("remittance\x00" + network + "\x00" + strings.ToLower(from) + "\x00" + strings.ToLower(nonce)))
	return "rem_" + hex.EncodeToString(sum[:16])
}

// systemDB is where remittances live: one namespace for every org, so one
// payment is one row whoever presents it.
func systemDB() *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(context.Background(), "system"))
}

func load(id string) (*remittance.Remittance, error) {
	db := systemDB()
	r := remittance.New(db)
	if err := r.Get(db.NewKey(r.Kind(), id, 0, nil)); err != nil {
		return nil, err
	}
	return r, nil
}

// update applies fn to the stored remittance and writes the result ONLY over
// the row it read (mixin.Model.Swap), reading again when another writer got
// there first. fn returns false when the row already says what it would make
// it say. This is how the request and the reconciler, on any number of
// replicas, move one remittance without either losing the other's change.
func update(id string, fn func(*remittance.Remittance) bool) (*remittance.Remittance, error) {
	for range mixin.Swaps {
		r, err := load(id)
		if err != nil {
			return nil, err
		}
		if !fn(r) {
			return r, nil
		}
		ok, err := r.Swap()
		if err != nil {
			return nil, err
		}
		if ok {
			return r, nil
		}
	}
	return nil, fmt.Errorf("gateway: remittance %s changed under %d reads in a row", id, mixin.Swaps)
}

// stamp is a time as a remittance's Due: 20-digit zero-padded unix seconds.
func stamp(t time.Time) string { return fmt.Sprintf("%020d", max(t.Unix(), 0)) }

// never is a Due no clock reaches: the row is not looked at again.
var never = strings.Repeat("9", 20)

// backoff is how long the reconciler waits before looking at a row that has
// been looked at tries times in a row with nothing changing: a minute,
// doubling, at most half an hour.
func backoff(tries int) time.Duration {
	if tries >= 5 {
		return 30 * time.Minute
	}
	return time.Minute << tries
}

// resend is how long a relayer transaction is given to show up mined before
// the rail asks whether the node still holds it, and sends again if not.
const resend = 2 * time.Minute

// bumpAfter is how long a relayer transaction may sit in the pool before it is
// replaced at the same nonce with higher fees.
const bumpAfter = 5 * time.Minute

// grace is how long past validBefore an unused authorization is still looked
// for, so a transaction mined just in time is not missed for a lagging node.
const grace = 10 * time.Minute

// Submit takes a signed payment from the caller and returns what became of it.
//
// A payment the token would take is relayed and comes back pending, with the
// relayer's transaction; one already used on chain comes back confirming or
// succeeded; one the token would refuse comes back as a 400 naming why. The
// outcome of a relayed payment is the chain's: read it again (GET) or present
// it again — either way it resolves to at most one credit. Presenting the
// same payment twice, at once or an hour apart, is one payment.
func (s *Service) Submit(ctx context.Context, org *organization.Organization, payer string, sub Submission) (View, error) {
	if err := s.caller(org, payer); err != nil {
		return View{}, err
	}
	if err := s.budget("submit", org.Name); err != nil {
		return View{}, err
	}
	c, err := verify(sub, s.cfg.Chains, s.cfg.Treasury, org.Name, payer)
	if err != nil {
		return View{}, err
	}
	l, err := s.ready(ctx, c.chain.ID)
	if err != nil {
		return View{}, err
	}
	r, err := s.claim(ctx, l, org, payer, c)
	if err != nil {
		return View{}, err
	}
	switch r.Status {
	case remittance.Pending, remittance.Refused:
		r, err = s.drive(ctx, l, r)
	case remittance.Confirming:
		r = s.confirm(ctx, l, r)
	}
	if r == nil {
		return View{}, err
	}
	return s.view(r), err
}

// claim records the payment, or finds the record a previous presentation of
// it made. Only a NEW payment is held to the validity window: a recorded one
// is answered with its record, so a client retrying later is told what
// happened rather than that its payment is now too short-lived. The insert is
// a swap over "nothing stored", so of two presentations racing on any number
// of replicas exactly one inserts and the other reads what it inserted.
func (s *Service) claim(ctx context.Context, l *link, org *organization.Organization, payer string, c checked) (*remittance.Remittance, error) {
	id := ID(c.chain.ID, c.auth.From, c.auth.Nonce)
	for range mixin.Swaps {
		r, err := load(id)
		switch {
		case err == nil:
			if r.Org != org.Name || r.Payer != payer || !same(r, c) {
				return nil, refuse(http.StatusConflict, ReasonConflict, "this nonce was already used by %s for a different payment; fetch new terms and sign their nonce", c.auth.From)
			}
			return r, nil
		case !errors.Is(err, datastore.ErrNoSuchEntity):
			return nil, fmt.Errorf("gateway: read remittance %s: %w", id, err)
		}
		now := s.now()
		if err := window(c, now); err != nil {
			return nil, err
		}
		if err := s.room(org.Name, c.cents, now); err != nil {
			return nil, err
		}
		// The blocks a use of this authorization can be in, at no fewer than one
		// block a second (every network here makes at most one), with a margin
		// for a lagging node: from when it became valid to when it stops.
		head, err := l.rpc().BlockNumber(ctx)
		if err != nil {
			return nil, shut(ReasonUnavailable, "The "+l.chain.Name+" network is not answering; present the same payment again shortly.", err)
		}
		back := uint64(now.Unix()) - c.after + 64
		ahead := c.before - uint64(now.Unix()) + 64
		r = remittance.New(systemDB())
		r.SetId(id)
		r.Org, r.Payer, r.Test, r.Rail = org.Name, payer, org.TestMode(), remittance.Rail
		r.Network, r.Asset = c.chain.ID, c.chain.USDC
		r.From, r.To, r.Value = c.auth.From, c.auth.To, c.auth.Value
		r.ValidAfter, r.ValidBefore, r.Nonce = c.auth.ValidAfter, c.auth.ValidBefore, c.auth.Nonce
		r.Salt, r.Signature, r.AmountCents = c.salt, c.sig, c.cents
		r.FromBlock = int64(head - min(back, head))
		r.ToBlock = int64(head + ahead)
		r.Scanned = r.FromBlock - 1
		r.Status = remittance.Pending
		// The request drives it now; the reconciler takes over only if the
		// request does not finish.
		r.Due = stamp(now.Add(time.Minute))
		ok, err := r.Swap()
		if err != nil {
			return nil, fmt.Errorf("gateway: record remittance %s: %w", id, err)
		}
		if ok {
			return r, nil
		}
	}
	return nil, fmt.Errorf("gateway: remittance %s changed under %d reads in a row", id, mixin.Swaps)
}

// same reports whether a stored remittance is this payment: the same signed
// message under the same salt. The signature is not compared — a second valid
// signature over one message is the same payment, not another.
func same(r *remittance.Remittance, c checked) bool {
	return r.Network == c.chain.ID && r.From == c.auth.From && r.To == c.auth.To &&
		r.Value == c.auth.Value && r.ValidAfter == c.auth.ValidAfter &&
		r.ValidBefore == c.auth.ValidBefore && r.Nonce == c.auth.Nonce && r.Salt == c.salt
}

// room checks a new payment fits what one org may have going: at most MaxOpen
// pending and MaxRefused refused-and-watched at once, and at most DayCents
// presented in a UTC day. The day's
// total is a tally taken as it is checked, so it counts every payment
// presented, whatever became of it, and two presented at once cannot both fit
// in room for one.
func (s *Service) room(org string, cents int64, now time.Time) error {
	var open []*remittance.Remittance
	if _, err := remittance.Query(systemDB()).Filter("Org=", org).Filter("Status=", remittance.Pending).
		Limit(MaxOpen).GetAll(&open); err != nil {
		return fmt.Errorf("gateway: count open payments: %w", err)
	}
	if len(open) >= MaxOpen {
		return refuse(http.StatusTooManyRequests, ReasonOpen, "this account has %d payments in progress; present another once one of them is confirmed", len(open))
	}
	var watched []*remittance.Remittance
	if _, err := remittance.Query(systemDB()).Filter("Org=", org).Filter("Status=", remittance.Refused).
		Limit(MaxRefused).GetAll(&watched); err != nil {
		return fmt.Errorf("gateway: count refused payments: %w", err)
	}
	if len(watched) >= MaxRefused {
		return refuse(http.StatusTooManyRequests, ReasonOpen, "this account has %d refused payments still valid; fund the wallet and present one of them again, or let them expire", len(watched))
	}
	id := "day\x00" + org + "\x00" + now.UTC().Format(time.DateOnly)
	db := systemDB()
	for range mixin.Swaps {
		t := remittance.NewTally(db)
		switch err := t.Get(db.NewKey(t.Kind(), id, 0, nil)); {
		case errors.Is(err, datastore.ErrNoSuchEntity):
			t = remittance.NewTally(db)
			t.SetId(id)
		case err != nil:
			return fmt.Errorf("gateway: read the day's tally: %w", err)
		}
		if t.Cents+cents > DayCents {
			return refuse(http.StatusTooManyRequests, ReasonDaily, "this account may present at most %d cents of USDC a UTC day; it has presented %d", DayCents, t.Cents)
		}
		t.Cents += cents
		ok, err := t.Swap()
		if err != nil {
			return fmt.Errorf("gateway: write the day's tally: %w", err)
		}
		if ok {
			return nil
		}
	}
	return fmt.Errorf("gateway: the day's tally changed under %d reads in a row", mixin.Swaps)
}

// owed is what a wallet's pending payments on a network come to, in base
// units: every payment the relayer could send from it, sent or waiting.
func owed(r *remittance.Remittance) (*big.Int, error) {
	var rows []*remittance.Remittance
	if _, err := remittance.Query(systemDB()).Filter("Network=", r.Network).Filter("From=", r.From).
		Filter("Status=", remittance.Pending).Limit(1000).GetAll(&rows); err != nil {
		return nil, err
	}
	total, self := new(big.Int), false
	for _, x := range rows {
		v, _ := new(big.Int).SetString(x.Value, 10)
		if v != nil {
			total.Add(total, v)
		}
		self = self || x.Id() == r.Id()
	}
	if !self {
		v, _ := new(big.Int).SetString(r.Value, 10)
		if v != nil {
			total.Add(total, v)
		}
	}
	return total, nil
}

// Get reads one of the caller's remittances. Another caller's id answers
// exactly as an id that names nothing.
func (s *Service) Get(ctx context.Context, org *organization.Organization, payer, id string) (View, error) {
	if org == nil || payer == "" {
		return View{}, refuse(http.StatusUnauthorized, ReasonCaller, "authentication required")
	}
	if !strings.HasPrefix(id, "rem_") {
		return View{}, errMissing
	}
	r, err := load(id)
	if err != nil {
		if errors.Is(err, datastore.ErrNoSuchEntity) {
			return View{}, errMissing
		}
		return View{}, fmt.Errorf("gateway: read remittance %s: %w", id, err)
	}
	if r.Org != org.Name || r.Payer != payer {
		return View{}, errMissing
	}
	return s.view(r), nil
}

// before is the authorization's validBefore.
func before(r *remittance.Remittance) time.Time {
	b, err := strconv.ParseInt(r.ValidBefore, 10, 64)
	if err != nil {
		return time.Unix(0, 0)
	}
	return time.Unix(b, 0)
}

// drive takes a pending or refused remittance as far as the chain lets it:
// to confirming when the chain shows it used, to failed when it can no longer
// be, to the relayer when the token would take it now and its wallet has no
// other payment in flight, and to refused when the token would not or the
// wallet cannot fund every payment pending from it. A step that cannot be
// decided yet — a node that does not answer, a transaction still in the pool,
// a wallet whose other payment is in flight — leaves the row as it is.
func (s *Service) drive(ctx context.Context, l *link, r *remittance.Remittance) (*remittance.Remittance, error) {
	if r.Rail != remittance.Rail {
		return r, nil // the Circle Gateway rail's row: never acted on here
	}
	used, err := l.used(ctx, r.From, r.Nonce)
	if err != nil {
		slog.Warn("gateway: authorizationState unread; the reconciler asks again", "remittance", r.Id(), "err", err)
		return r, nil
	}
	if used {
		return s.settle(ctx, l, r), nil
	}
	now := s.now()
	switch {
	case !now.Before(before(r).Add(grace)):
		return s.fail(r, ReasonExpired), nil
	case !now.Before(before(r)):
		return r, nil // too late to send; looked at until grace has passed
	}
	if inFlight(r) {
		if now.Sub(r.SentAt) < resend {
			return r, nil // the relayer's transaction has not had time to land
		}
		last := r.Sends[len(r.Sends)-1]
		rc, err := l.rpc().Receipt(ctx, last)
		if err != nil {
			return r, nil
		}
		if rc == nil {
			known, err := l.rpc().Known(ctx, last)
			switch {
			case err != nil:
				return r, nil
			case known && now.Sub(r.SentAt) >= bumpAfter:
				return s.send(ctx, l, r, true) // stuck: the same nonce, higher fees
			case known:
				return r, nil // in the pool: it lands, or it is replaced
			}
		}
		// Mined and failed, or dropped: ask again what the token would say now.
	}

	// The wallet must fund every payment pending from it, this one included,
	// or the relayer pays gas for transfers that revert.
	total, err := owed(r)
	if err != nil {
		return r, nil
	}
	held, err := l.balance(ctx, r.From)
	if err != nil {
		return r, nil
	}
	if held.Cmp(total) < 0 {
		return s.refused(r, ReasonBalance)
	}
	// One payment in flight per wallet: the rest wait their turn.
	if mine, err := s.lane(r); err != nil || !mine {
		return r, nil
	}

	data, err := transferCall(r)
	if err != nil {
		slog.Error("RECONCILE: gateway: a recorded payment does not encode; it cannot be relayed", "remittance", r.Id(), "err", err)
		return s.fail(r, ReasonPayload), nil
	}
	if _, err := l.rpc().Call(ctx, s.cfg.RelayerAddress(), l.chain.USDC, data); err != nil {
		reason, final := revert(err)
		switch reason {
		case "":
			slog.Warn("gateway: simulation unanswered; the reconciler asks again", "remittance", r.Id(), "err", err)
			return r, nil
		case "used":
			return s.settle(ctx, l, r), nil
		case ReasonEarly, "paused":
			return r, nil
		case ReasonExpired:
			return s.fail(r, ReasonExpired), nil
		}
		if final {
			// The signature verified under the domain ready() checked against the
			// token, and the token refuses it: the two disagree, which is ours
			// to find, not the payer's.
			slog.Error("RECONCILE: gateway: the token refused a signature the rail verified", "remittance", r.Id(), "network", r.Network, "err", err)
		}
		return s.refused(r, reason)
	}
	return s.send(ctx, l, r, false)
}

// send has the relayer sign transferWithAuthorization for r, records the
// transaction on the row, and only then broadcasts it: every transaction Hanzo's
// key signs is named in the record before it can exist anywhere else. With
// replace it is the replacement of r's last send, stuck in the pool: the same
// relayer nonce, higher fees, so at most one of the two can be mined. The
// relayer key is leased across replicas for the draft and the broadcast, so two
// replicas never draft the same nonce.
func (s *Service) send(ctx context.Context, l *link, r *remittance.Remittance, replace bool) (*remittance.Remittance, error) {
	s.relay.Lock()
	defer s.relay.Unlock()

	// Another replica, or the reconciler here, may have sent meanwhile.
	fresh, err := load(r.Id())
	if err != nil {
		return r, nil
	}
	if !fresh.SentAt.Equal(r.SentAt) || (fresh.Status != remittance.Pending && fresh.Status != remittance.Refused) {
		return fresh, nil
	}
	r = fresh
	if took, err := s.lease(r.Network); err != nil || !took {
		return r, nil // another replica is drafting; the reconciler sends it
	}
	defer s.release(r.Network)

	var prev *evm.Fees
	if replace {
		tip, ok1 := new(big.Int).SetString(r.RelayTip, 10)
		cap, ok2 := new(big.Int).SetString(r.RelayCap, 10)
		if !ok1 || !ok2 {
			return r, nil
		}
		prev = &evm.Fees{Nonce: uint64(r.RelayNonce), Tip: tip, Cap: cap}
	}
	raw, hash, fees, err := s.sign(ctx, l, r, prev)
	switch {
	case errors.Is(err, errNoFee):
		slog.Error("RECONCILE: gateway: the relayer cannot pay the gas; nothing is relayed until it is funded", "network", r.Network, "relayer", s.cfg.RelayerAddress(), "err", err)
		return r, shut(ReasonRelayer, "The relayer cannot pay the network fee right now. Your payment is recorded and nothing has moved; it is sent once the relayer is funded, or present it again later.", err)
	case err != nil:
		slog.Warn("gateway: relayer transaction not drafted; the reconciler tries again", "remittance", r.Id(), "err", err)
		return r, nil
	}

	at := s.now()
	r, err = update(r.Id(), func(x *remittance.Remittance) bool {
		if !x.SentAt.Equal(fresh.SentAt) || (x.Status != remittance.Pending && x.Status != remittance.Refused) {
			return false
		}
		x.Sends = append(x.Sends, hash)
		x.SentAt, x.Relayer = at, s.cfg.RelayerAddress()
		x.RelayNonce, x.RelayTip, x.RelayCap = int64(fees.Nonce), fees.Tip.String(), fees.Cap.String()
		x.Status, x.Reason, x.Tries = remittance.Pending, "", 0
		x.Due = stamp(at.Add(time.Minute))
		return true
	})
	if err != nil || !r.SentAt.Equal(at) {
		return r, nil // not recorded, or another writer moved it: nothing is broadcast
	}
	slog.Info("audit: gateway relayed a USDC authorization",
		"remittance", r.Id(), "network", r.Network, "tx", hash, "relayer", r.Relayer, "nonce", fees.Nonce,
		"replaces", replace, "from", r.From, "value", r.Value, "cents", r.AmountCents, "org", r.Org, "payer", r.Payer, "send", len(r.Sends))

	got, err := l.evm.Broadcast(ctx, raw)
	switch {
	case err != nil:
		slog.Warn("gateway: broadcast unanswered; the transaction is looked up, and sent again if the node does not hold it", "remittance", r.Id(), "tx", hash, "err", err)
	case !strings.EqualFold(got, hash):
		slog.Error("RECONCILE: gateway: the node named a different hash for the relayer's transaction", "remittance", r.Id(), "built", hash, "node", got)
	}
	return r, nil
}

// settle follows a remittance whose authorization the chain shows used: it
// finds what used it. The transaction paid the treasury exactly this payment —
// confirming from here — or the nonce went to another authorization, or the
// payer canceled it, and this one can never pay. A use not found once its
// whole window is searched fails too: nothing is credited that cannot be
// shown. Who submitted it does not matter.
func (s *Service) settle(ctx context.Context, l *link, r *remittance.Remittance) *remittance.Remittance {
	u, err := s.locate(ctx, l, r)
	switch {
	case err != nil:
		slog.Warn("gateway: the use of an authorization was not read; the reconciler asks again", "remittance", r.Id(), "err", err)
		return r
	case u == nil:
		if head, err := l.rpc().BlockNumber(ctx); err == nil && s.searched(r, head) {
			slog.Error("RECONCILE: gateway: the token says the authorization is spent and its window shows no use or cancel; failed, uncredited", "remittance", r.Id(), "from", r.From, "nonce", r.Nonce, "org", r.Org)
			return s.fail(r, ReasonUnaccounted)
		}
		return r // used, and the blocks searched do not show where yet
	case u.canceled:
		return s.fail(r, ReasonCanceled)
	case !u.paid:
		slog.Error("RECONCILE: gateway: the nonce was spent on a different authorization; this payment can never land", "remittance", r.Id(), "tx", u.tx, "from", r.From, "nonce", r.Nonce, "org", r.Org)
		return s.fail(r, ReasonConflict)
	}
	out, err := update(r.Id(), func(x *remittance.Remittance) bool {
		if x.Status != remittance.Pending && x.Status != remittance.Refused {
			return false
		}
		x.Status, x.Reason, x.Tries = remittance.Confirming, "", 0
		x.TxHash, x.Block = u.tx, int64(u.block)
		x.Due = stamp(s.now().Add(time.Minute))
		return true
	})
	if err != nil {
		return r
	}
	if out.Status != remittance.Confirming {
		return out
	}
	return s.confirm(ctx, l, out)
}

// confirm credits a confirming remittance once its transaction is past the
// point the chain may take it back, and not before: at least Confirmations
// deep, and in a block at or below the node's SAFE head — its FINALIZED head
// from FinalCents up. At that point the receipt is read again, and a SECOND
// provider (the witness) must show the same use in the same block, past the
// same head. A reorganisation that took the transaction out or moved it sends
// the row back to pending; a witness that disagrees holds the credit and says
// so, for an operator.
func (s *Service) confirm(ctx context.Context, l *link, r *remittance.Remittance) *remittance.Remittance {
	if r.Credited {
		return s.land(ctx, r)
	}
	tag := "safe"
	if r.AmountCents >= FinalCents {
		tag = "finalized"
	}
	head, err := l.rpc().BlockNumber(ctx)
	if err != nil {
		return r
	}
	mark, err := l.rpc().Head(ctx, tag)
	if err != nil {
		return r
	}
	depth := int64(head) - r.Block + 1
	if depth < l.chain.Confirmations || uint64(r.Block) > mark {
		out, err := update(r.Id(), func(x *remittance.Remittance) bool {
			if x.Status != remittance.Confirming || x.Confirmations == max(depth, 0) {
				return false
			}
			x.Confirmations = max(depth, 0)
			return true
		})
		if err != nil {
			return r
		}
		return out
	}
	rc, err := l.rpc().Receipt(ctx, r.TxHash)
	if err != nil {
		return r
	}
	u := s.spent(l, r, r.TxHash, rc)
	if u == nil || !u.paid || int64(u.block) != r.Block {
		slog.Warn("gateway: the transaction that used an authorization is no longer where it was; following it again", "remittance", r.Id(), "tx", r.TxHash, "block", r.Block)
		out, err := update(r.Id(), func(x *remittance.Remittance) bool {
			if x.Status != remittance.Confirming || x.Credited {
				return false
			}
			x.Status, x.TxHash, x.Block, x.Confirmations = remittance.Pending, "", 0, 0
			return true
		})
		if err != nil {
			return r
		}
		return out
	}
	wrc, err := l.witness.Receipt(ctx, r.TxHash)
	if err != nil {
		return r
	}
	if w := s.spent(l, r, r.TxHash, wrc); w == nil || !w.paid || int64(w.block) != r.Block {
		slog.Error("RECONCILE: gateway: the witness does not show this use where the node does; not credited", "remittance", r.Id(), "tx", r.TxHash, "block", r.Block, "network", r.Network)
		return r
	}
	if wmark, err := l.witness.Head(ctx, tag); err != nil || uint64(r.Block) > wmark {
		return r
	}
	// OWED from here: written before the ledger is asked, so a landing that
	// does not finish is finished by the reconciler.
	out, err := update(r.Id(), func(x *remittance.Remittance) bool {
		if x.Status != remittance.Confirming || x.Credited {
			return false
		}
		x.Credited, x.Confirmations = true, depth
		return true
	})
	if err != nil || !out.Credited {
		return r
	}
	return s.land(ctx, out)
}

// refused records that the token would refuse the payment now, and answers it
// as a 400 naming why. Nothing was sent and nothing is credited. The chain is
// still watched for its use until it expires: the payer may hand the same
// authorization to the token directly, and the money would then be the
// treasury's.
func (s *Service) refused(r *remittance.Remittance, reason string) (*remittance.Remittance, error) {
	r, err := update(r.Id(), func(x *remittance.Remittance) bool {
		if x.Status != remittance.Pending && x.Status != remittance.Refused {
			return false
		}
		if x.Status == remittance.Refused && x.Reason == reason {
			return false
		}
		x.Status, x.Reason, x.Tries = remittance.Refused, reason, 0
		x.Due = stamp(s.now().Add(backoff(0)))
		return true
	})
	if err != nil {
		return nil, err
	}
	if r.Status != remittance.Refused {
		return r, nil // another writer moved it meanwhile: that answer stands
	}
	msg := "The USDC contract would refuse this payment now (" + reason + "). Nothing moved."
	switch reason {
	case ReasonBalance:
		msg = "The paying wallet holds less USDC than this payment. Nothing moved. Fund the wallet, then present the same payment again."
	case ReasonBlocked:
		msg = "The USDC contract does not let one of these addresses move USDC. Nothing moved."
	}
	return r, invalid(reason, "%s", msg)
}

// fail records that a remittance can never pay the treasury. Final; nothing
// was credited, because a credit is made only on a confirmed use.
func (s *Service) fail(r *remittance.Remittance, reason string) *remittance.Remittance {
	out, err := update(r.Id(), func(x *remittance.Remittance) bool {
		if x.Status == remittance.Failed || x.Status == remittance.Succeeded || x.Status == remittance.Confirming || x.Credited {
			return false
		}
		x.Status, x.Reason, x.Due = remittance.Failed, reason, never
		return true
	})
	if err != nil {
		return r
	}
	return out
}
