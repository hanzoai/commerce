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
	"strconv"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/remittance"
	"github.com/hanzoai/commerce/payment/providers/circle"
	"github.com/hanzoai/commerce/util/nscontext"
)

// View is a remittance as its payer reads it. Credited means the money is in
// the payer's account; Deferred means it will be once Gateway confirms the
// payment onchain (see Hold).
type View struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Network     string `json:"network"`
	From        string `json:"from"`
	Value       string `json:"value"`
	AmountCents int64  `json:"amountCents"`
	Credited    bool   `json:"credited"`
	Deferred    bool   `json:"deferred,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Payment     string `json:"payment,omitempty"`
	TxHash      string `json:"txHash,omitempty"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

func view(r *remittance.Remittance) View {
	return View{
		ID: r.Id(), Status: r.Status, Network: r.Network, From: r.From, Value: r.Value,
		AmountCents: r.AmountCents, Credited: r.Landed && !r.Reversed, Deferred: r.Deferred && !r.Credited,
		Reason: r.Reason, Payment: r.Payment, TxHash: r.TxHash,
		CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: r.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// ID is a payment's storage id: a pure function of its identity on the chain.
// Gateway refuses a second use of a (payer address, nonce) on a network, so
// these three name one payment and nothing else names it.
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

// Submit takes a signed payment from the caller and returns what became of it.
//
// A payment Gateway accepted comes back accepted (credited, or deferred until
// Gateway confirms it); one Gateway refused comes back as a 400 carrying
// Gateway's reason; one whose outcome is not yet known comes back pending, with
// no error — the money may have moved, so the caller reads it again (GET) or
// presents it again, and either way it resolves to at most one credit.
// Presenting the same payment twice, at once or a day apart, is one payment.
func (s *Service) Submit(ctx context.Context, org *organization.Organization, payer string, sub Submission) (View, error) {
	if err := s.caller(org, payer); err != nil {
		return View{}, err
	}
	nets, err := s.networks(ctx)
	if err != nil {
		return View{}, err
	}
	c, err := verify(sub, nets, s.cfg.Treasury, org.Name, payer)
	if err != nil {
		return View{}, err
	}
	r, err := s.claim(org, payer, c)
	if err != nil {
		return View{}, err
	}
	switch r.Status {
	case remittance.Accepted:
		return view(s.credit(ctx, r)), nil
	case remittance.Pending, remittance.Refused:
		if r, err = s.drive(ctx, r); err != nil {
			return View{}, err
		}
	}
	return view(r), nil
}

// claim records the payment, or finds the record a previous presentation of
// it made. Only a NEW payment is held to the validity window: a recorded one
// is answered with its record, so a client retrying hours later is told what
// happened rather than that its payment is now too short-lived. The insert is
// a swap over "nothing stored", so of two presentations racing on any number
// of replicas exactly one inserts and the other reads what it inserted.
func (s *Service) claim(org *organization.Organization, payer string, c checked) (*remittance.Remittance, error) {
	id := ID(c.net.id, c.auth.From, c.auth.Nonce)
	for range mixin.Swaps {
		r, err := load(id)
		switch {
		case err == nil:
			if r.Org != org.Name || r.Payer != payer || !same(r, c) {
				return nil, refuse(409, ReasonConflict, "this nonce was already used by %s for a different payment; fetch new terms and sign their nonce", c.auth.From)
			}
			return r, nil
		case !errors.Is(err, datastore.ErrNoSuchEntity):
			return nil, fmt.Errorf("gateway: read remittance %s: %w", id, err)
		}
		if err := window(c, s.now()); err != nil {
			return nil, err
		}
		r = remittance.New(systemDB())
		r.SetId(id)
		r.Org, r.Payer, r.Test = org.Name, payer, org.TestMode()
		r.Network, r.Asset, r.Contract = c.net.id, c.net.asset, c.net.domain.VerifyingContract
		r.From, r.To, r.Value = c.auth.From, c.auth.To, c.auth.Value
		r.ValidAfter, r.ValidBefore, r.Nonce = c.auth.ValidAfter, c.auth.ValidBefore, c.auth.Nonce
		r.Salt, r.Signature, r.AmountCents = c.salt, c.sig, c.cents
		r.Status = remittance.Pending
		// The request drives it now; the reconciler takes over only if the
		// request does not finish.
		r.Due = stamp(s.now().Add(time.Minute))
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
	return r.Network == c.net.id && r.From == c.auth.From && r.To == c.auth.To &&
		r.Value == c.auth.Value && r.ValidAfter == c.auth.ValidAfter &&
		r.ValidBefore == c.auth.ValidBefore && r.Nonce == c.auth.Nonce && r.Salt == c.salt
}

// Get reads one of the caller's remittances. Another caller's id answers
// exactly as an id that names nothing.
func (s *Service) Get(ctx context.Context, org *organization.Organization, payer, id string) (View, error) {
	if org == nil || payer == "" {
		return View{}, refuse(401, ReasonCaller, "authentication required")
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
	return view(r), nil
}

// requirements is what the treasury asks for, rebuilt from the record so a
// payment can be re-driven whatever Gateway's network table says today. PayTo
// is the CONFIGURED treasury, never the payment's own `to`: verify already
// refused any other payee, and stating the treasury here makes Gateway refuse
// one too (address_mismatch) should that check ever be lost.
func (s *Service) requirements(r *remittance.Remittance) circle.Requirements {
	return circle.Requirements{
		Scheme: circle.Scheme, Network: r.Network, Asset: r.Asset, Amount: r.Value,
		PayTo: s.cfg.Treasury, MaxTimeoutSeconds: int64(MaxValidity / time.Second),
		Extra: circle.Signing{Name: circle.Name, Version: circle.Release, VerifyingContract: r.Contract},
	}
}

func resource(r *remittance.Remittance) circle.Resource {
	return circle.Resource{
		URL:         "/v1/billing/crypto/gateway/" + r.Id(),
		Description: "Hanzo balance top-up",
		MimeType:    "application/json",
	}
}

func payload(r *remittance.Remittance) circle.Payload {
	return circle.Payload{
		Authorization: circle.Authorization{
			From: r.From, To: r.To, Value: r.Value,
			ValidAfter: r.ValidAfter, ValidBefore: r.ValidBefore, Nonce: r.Nonce,
		},
		Signature: r.Signature,
	}
}

// past reports whether the payment's validBefore has passed: Gateway would
// refuse to settle it now, whatever happened to it before.
func (s *Service) past(r *remittance.Remittance) bool {
	before, err := strconv.ParseInt(r.ValidBefore, 10, 64)
	return err != nil || !s.now().Before(time.Unix(before, 0))
}

// expired reports whether a payment Gateway never showed as accepted can no
// longer have been: past its validBefore by long enough that any acceptance
// Gateway made in time is readable.
func (s *Service) expired(r *remittance.Remittance) bool {
	before, err := strconv.ParseInt(r.ValidBefore, 10, 64)
	return err != nil || s.now().After(time.Unix(before, 0).Add(time.Hour))
}

// errNonce is the answer when Gateway holds ANOTHER payment under this payer's
// nonce: this one can never settle, so it is failed for good.
var errNonce = refuse(409, ReasonConflict, "Circle Gateway already holds a different payment under this nonce; it can never settle. Fetch new terms and sign their nonce.")

// drive takes a pending or refused remittance to Gateway and records the
// answer.
//
// A remittance that may already be at Gateway — it was sent before, or its
// window has passed — is LOOKED UP first: an earlier send may have been
// accepted with nobody hearing the answer, or the payer may have handed the
// same authorization to Gateway directly, and Gateway, not this process,
// knows. Only when Gateway holds nothing for (payer, nonce) is it sent again,
// which is safe because Gateway refuses a second use of a nonce — so however
// many replicas drive one remittance, Gateway accepts it at most once, and the
// credit is keyed on the remittance, so it lands at most once.
//
// A refusal is believed only when nothing about the remittance is unknown. The
// first send's refusal is the whole truth; so is a refusal of a payment whose
// every earlier send was refused. But a re-send of a payment whose earlier
// send went unanswered may be refused BECAUSE that send was accepted — its
// balance locked, its validity window now short — so that refusal is not
// recorded: the remittance stays pending until Gateway shows the payment or it
// expires unseen.
func (s *Service) drive(ctx context.Context, r *remittance.Remittance) (*remittance.Remittance, error) {
	if r.Sent || s.past(r) {
		mine, other, err := s.find(ctx, r)
		switch {
		case err != nil:
			return r, nil // unknown: stays as it is; the reconciler asks again
		case mine != nil:
			return s.adopt(ctx, r, *mine)
		case other:
			r, err := s.fail(ctx, r, ReasonConflict)
			if err != nil {
				return nil, err
			}
			return r, errNonce
		case s.expired(r):
			return s.fail(ctx, r, "expired")
		case s.past(r):
			return r, nil // Gateway would refuse it; keep looking until it expires
		}
	}

	var unanswered bool
	r, err := update(r.Id(), func(x *remittance.Remittance) bool {
		if x.Status != remittance.Pending && x.Status != remittance.Refused {
			return false
		}
		unanswered = x.Status == remittance.Pending && x.Sent
		if x.Sent {
			return false
		}
		x.Sent = true
		return true
	})
	if err != nil {
		return nil, err
	}
	if r.Status != remittance.Pending && r.Status != remittance.Refused {
		return r, nil // another writer settled it while this one read
	}
	refusal := func(reason, detail string) (*remittance.Remittance, error) {
		if unanswered {
			slog.Warn("gateway: a re-send was refused while an earlier send is unanswered; pending until Gateway shows it or it expires", "remittance", r.Id(), "reason", reason, "detail", detail)
			return r, nil
		}
		return s.refused(r, reason, detail)
	}

	st, err := s.client.Settle(ctx, resource(r), s.requirements(r), payload(r))
	switch {
	case err != nil && circle.Refused(err):
		return refusal(ReasonRejected, err.Error())
	case err != nil:
		slog.Warn("gateway: settle outcome unknown", "remittance", r.Id(), "err", err)
		return s.unknown(r)
	case st.Success && st.Transaction != "" && (st.Payer == "" || strings.EqualFold(st.Payer, r.From)):
		return s.accept(ctx, r, circle.Payment{ID: st.Transaction, Status: circle.Received})
	case st.Success:
		slog.Error("RECONCILE: gateway: settle answered success without a usable payment; following it by nonce", "remittance", r.Id(), "transaction", st.Transaction, "payer", st.Payer, "from", r.From)
		return s.unknown(r)
	case st.Reason == "nonce_already_used":
		// Used by whom is exactly the question: an earlier send of THIS
		// payment, or another payment under the same nonce. Gateway says which.
		mine, other, ferr := s.find(ctx, r)
		switch {
		case ferr == nil && mine != nil:
			return s.adopt(ctx, r, *mine)
		case ferr == nil && other:
			r, err := s.fail(ctx, r, ReasonConflict)
			if err != nil {
				return nil, err
			}
			return r, errNonce
		}
		return s.unknown(r) // not visible yet: the reconciler keeps looking
	case st.Reason == "" || st.Reason == "unexpected_error":
		return s.unknown(r)
	default:
		return refusal(st.Reason, "")
	}
}

// find asks Gateway what it holds under this remittance's (network, payer,
// nonce). mine is the payment that IS this remittance — same payer, payee,
// amount and nonce. other reports a payment under the nonce that is not this
// one: another payee or amount, which means this one can never settle.
func (s *Service) find(ctx context.Context, r *remittance.Remittance) (mine *circle.Payment, other bool, err error) {
	pays, err := s.client.Find(ctx, r.Network, r.From, r.Nonce)
	if err != nil {
		return nil, false, err
	}
	for i := range pays {
		switch {
		case matches(r, pays[i]):
			return &pays[i], false, nil
		case strings.EqualFold(pays[i].From, r.From) && strings.EqualFold(pays[i].Nonce, r.Nonce) &&
			(pays[i].SendingNetwork == "" || pays[i].SendingNetwork == r.Network):
			other = true
		}
	}
	return nil, other, nil
}

// matches reports whether Gateway's payment record is this remittance: same
// payer, payee, amount and nonce, on the same network when Gateway says.
func matches(r *remittance.Remittance, p circle.Payment) bool {
	return strings.EqualFold(p.From, r.From) && strings.EqualFold(p.To, r.To) &&
		sameAmount(p.Amount, r.Value) && strings.EqualFold(p.Nonce, r.Nonce) &&
		(p.SendingNetwork == "" || p.SendingNetwork == r.Network)
}

// adopt records what Gateway's payment record says of this remittance.
func (s *Service) adopt(ctx context.Context, r *remittance.Remittance, p circle.Payment) (*remittance.Remittance, error) {
	if p.Status == circle.Failed {
		return s.fail(ctx, r, "failed")
	}
	return s.accept(ctx, r, p)
}

// accept records a payment Gateway holds — accepted, confirmed onchain, or
// completed — and credits it.
//
// Gateway's states are not equal. received and batched mean the payer's
// balance is LOCKED; confirmed means the batch carrying it landed onchain;
// only completed is final. A credit made on a lock counts against the payer's
// Hold until the payment is confirmed, so the rail never extends more than
// MaxCents of credit Gateway has not confirmed to one payer; past that, a
// payment is credited when it is confirmed.
func (s *Service) accept(ctx context.Context, r *remittance.Remittance, p circle.Payment) (*remittance.Remittance, error) {
	onchain := p.Status == circle.Confirmed || p.Status == circle.Completed
	if onchain {
		// Confirmed money needs no room in the hold. Released FIRST, so a
		// process that stops before the row is written releases again next
		// time rather than never.
		if err := s.release(r); err != nil {
			return r, nil
		}
	}
	status := remittance.Accepted
	if p.Status == circle.Completed {
		status = remittance.Settled
	}
	r, err := update(r.Id(), func(x *remittance.Remittance) bool {
		if x.Status == remittance.Failed || x.Status == remittance.Settled {
			return false
		}
		if x.Status == status && x.Payment == p.ID && x.State == p.Status && (p.TxHash == "" || x.TxHash == p.TxHash) {
			return false
		}
		if x.Status != status || x.State != p.Status {
			x.Tries = 0 // progress: look again soon
		}
		x.Status, x.Payment, x.State, x.Reason = status, p.ID, p.Status, ""
		if p.TxHash != "" {
			x.TxHash = p.TxHash
		}
		if status == remittance.Settled {
			x.SettledAt = s.now()
		}
		x.Due = stamp(s.now().Add(backoff(x.Tries)))
		return true
	})
	if err != nil {
		return nil, err
	}
	if onchain {
		// A pass holding an older read of this row can have reserved room after the
		// release above and before this write, its own swap having seen the row
		// still locked. The row is onchain now, so whatever it reserved is released.
		_ = s.release(r)
	}
	if r.Status == remittance.Accepted || r.Status == remittance.Settled {
		r = s.credit(ctx, r)
	}
	return r, nil
}

// unknown leaves a remittance pending for the reconciler. A refused one that
// was presented again and lost its answer becomes pending: the money may have
// moved this time.
func (s *Service) unknown(r *remittance.Remittance) (*remittance.Remittance, error) {
	return update(r.Id(), func(x *remittance.Remittance) bool {
		if x.Status != remittance.Refused {
			return false
		}
		x.Status, x.Reason = remittance.Pending, ""
		return true
	})
}

// refused records Gateway's definitive refusal and answers it as a 400 with
// Gateway's reason. Nothing was locked and nothing is credited. The
// reconciler still looks the nonce up until the payment expires, because a
// signed payment Gateway refused today can be handed to Gateway directly
// tomorrow, and the money would then be the treasury's.
func (s *Service) refused(r *remittance.Remittance, reason, detail string) (*remittance.Remittance, error) {
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
		return r, nil // accepted by another writer meanwhile: that answer stands
	}
	msg := "Circle Gateway refused the payment: " + reason
	if detail != "" {
		msg += " (" + detail + ")"
	}
	if reason == "insufficient_balance" {
		msg += ". Deposit USDC into your Gateway balance on this network, then present the same payment again."
	}
	return r, invalid(reason, "%s", msg)
}

// fail records that Gateway will never settle this remittance. The books are
// put right FIRST — the hold released, any credit taken back — and the row is
// marked failed only once they are, so a failure that cannot be reversed yet
// stays open and the reconciler finishes it.
func (s *Service) fail(ctx context.Context, r *remittance.Remittance, reason string) (*remittance.Remittance, error) {
	fresh, err := load(r.Id())
	if err != nil {
		return r, nil
	}
	r = fresh
	if r.Status == remittance.Failed || r.Status == remittance.Settled {
		return r, nil
	}
	if err := s.release(r); err != nil {
		return r, nil
	}
	if r.Credited && !r.Reversed {
		if r = s.reverse(ctx, r); !r.Reversed {
			return r, nil
		}
	}
	return markFailed(r.Id(), reason)
}

// markFailed is the last write of fail. A credit can be marked owed between
// fail's check and this write (another replica's credit() got there), and a
// failed row is never looked at again, so the row ends failed only while it
// holds no credit still owed back; otherwise it stays open for the reconciler
// to reverse on its next pass.
func markFailed(id, reason string) (*remittance.Remittance, error) {
	return update(id, func(x *remittance.Remittance) bool {
		if x.Status == remittance.Failed || x.Status == remittance.Settled || (x.Credited && !x.Reversed) {
			return false
		}
		x.Status, x.Reason, x.Due = remittance.Failed, reason, never
		if reason == "failed" {
			x.State = circle.Failed
		}
		return true
	})
}

// Circle may render the same base-unit amount with leading zeroes. Reject
// signs, decimals and exponents; compare the two unsigned integers by value.
func sameAmount(a, b string) bool {
	parse := func(s string) (*big.Int, bool) {
		// 78 digits hold 2^256; a longer string is not an amount, and parsing one
		// costs time that grows with its square.
		if s == "" || len(s) > 78 {
			return nil, false
		}
		for _, c := range s {
			if c < '0' || c > '9' {
				return nil, false
			}
		}
		return new(big.Int).SetString(s, 10)
	}
	x, ok := parse(a)
	if !ok {
		return false
	}
	y, ok := parse(b)
	return ok && x.Cmp(y) == 0
}
