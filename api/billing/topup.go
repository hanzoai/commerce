package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/mintauth"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/treasury"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// Sentinel errors so callers (Topup HTTP handler, auto-recharge cron) can map a
// charge outcome to the right HTTP status / log without re-deriving it.
var (
	errNoProcessor            = errors.New("no payment processor available")
	errChargedButCreditFailed = errors.New("charge succeeded but balance credit failed")
	errAmountOutOfBounds      = errors.New("amount outside the permitted range")
	errGuardUnavailable       = errors.New("idempotency guard unavailable")
	errChargeInFlight         = errors.New("charge already in progress")

	// errMethodUnchargeable is a saved row with no chargeable card behind it —
	// the vaulting half-completed, so the record exists and the instrument does
	// not. Distinct from the miss because the customer can fix this one by
	// adding the card again.
	errMethodUnchargeable = errors.New("saved payment method has no chargeable card")
)

// unasked reports a chargeAndCredit failure refused before the processor was
// asked to charge — out of bounds, no guard, already in flight, no processor —
// so the card saw nothing. Every other failure reached the processor.
func unasked(err error) bool {
	return errors.Is(err, errAmountOutOfBounds) || errors.Is(err, errGuardUnavailable) ||
		errors.Is(err, errChargeInFlight) || errors.Is(err, errNoProcessor)
}

// The classes a top-up can fail in, published so a caller outside this package
// answers them the way the endpoint answers them. Each is a different thing for the
// asker to do — fix the request, retry later, use another card, call support —
// which is why they are separate questions and not one "it failed".
//
// Two of the classes are the package's and not this file's: a request naming no
// card is IsMethodRefused, and a card that is not the paying subject's is
// IsMethodNotFound. Reusing them is what keeps "not there, or not yours" a
// single answer everywhere, which is the whole reason it cannot be probed.

// IsTopupOutOfBounds reports whether the amount sits outside the server-side
// [min,max]. The bounds are the endpoint's to state, since it is the endpoint that
// knows how to say them; this only reports that they were crossed.
func IsTopupOutOfBounds(err error) bool { return errors.Is(err, errAmountOutOfBounds) }

// IsMethodUnchargeable reports whether the saved method carries no chargeable
// card. It is the package's answer rather than this file's: the card subscribe
// path refuses the same row for the same reason and says the same sentence, and
// two names for one condition is how two endpoints come to disagree about it.
func IsMethodUnchargeable(err error) bool { return errors.Is(err, errMethodUnchargeable) }

// IsTopupNoProcessor reports whether this org has no payment processor that can
// take the money — a configuration state, not a decline.
func IsTopupNoProcessor(err error) bool { return errors.Is(err, errNoProcessor) }

// IsTopupInFlight reports whether an identical top-up is already running under
// the same key. The answer is to wait for it, never to charge again.
func IsTopupInFlight(err error) bool { return errors.Is(err, errChargeInFlight) }

// IsTopupGuardUnavailable reports whether the idempotency guard could not be
// consulted. Nothing was charged: the guard fails CLOSED, because a store that
// cannot tell a first attempt from a retry cannot protect a reusable saved card.
func IsTopupGuardUnavailable(err error) bool { return errors.Is(err, errGuardUnavailable) }

// IsTopupUncredited reports the one failure where MONEY MOVED: the card settled
// and the ledger did not follow. It is not retryable — a same-key retry is
// refused as in flight rather than charging twice — and it is the class that
// needs a person, which is why uncredited has already raised the alarm by the
// time a caller sees this.
func IsTopupUncredited(err error) bool { return errors.Is(err, errChargedButCreditFailed) }

// receipt is what a completed charge seals into its idempotency guard, so a
// replay returns the SAME answer instead of moving money a second time.
//
// ProcessorRef is the processor's reference for the charge and AmountCents what it
// charged: the key and the amount a ledger credits the payment under.
type receipt struct {
	TransactionId string `json:"transactionId"`
	BalanceCents  int64  `json:"balanceCents"`
	ProcessorRef  string `json:"processorRef,omitempty"`
	AmountCents   int64  `json:"amountCents,omitempty"`
}

type topupRequest struct {
	// No userId: the credited subject is the caller's own signed identity
	// (userBillingKey → account.Payer), never a body field, so a request can not
	// steer where the money lands. Same rule as the token endpoint.
	PaymentMethodID string `json:"paymentMethodId"`
	AmountCents     int64  `json:"amountCents"`
	Currency        string `json:"currency,omitempty"`
}

// chargeAndCredit charges a saved payment method through the org's processor
// registry (processorsForOrg) and, on success, credits the user's prepaid balance
// with a Deposit transaction. Returns the charge's receipt: the deposit
// transaction id, the new balance, the processor's reference and the amount.
//
// This is the single charge primitive reused by both the on-session top-up
// (TopupCard) and the off-session auto-recharge cron. For a Square card-on-file
// the SourceID is the saved card id (pm.ProviderRef) and CustomerID must be the
// Square customer id — a card-on-file is only chargeable in its customer's
// context (fall back to the org slug for legacy methods saved before card-on-file).
//
// It takes a context and the collector rather than a request: the cron reaching
// it has neither, and a primitive that needed one could only ever move money for
// somebody holding a browser.
//
// how says whether the payer is there: the on-session top-up may answer a
// *processor.ActionRequired for the page to finish, the off-session reload never
// does — its bank's demand is a decline.
func chargeAndCredit(ctx context.Context, ev *events.Client, org *organization.Organization, db *datastore.Datastore, pm *paymentmethod.PaymentMethod, amountCents int64, cur currency.Type, userId, idemKey, description string, how cardCharge) (receipt, error) {
	// Server-authoritative amount bounds, enforced HERE so both callers inherit
	// them: the HTTP top-up (where a scripted request bypasses the browser cap)
	// and the auto-recharge cron (where a mis-set config would otherwise charge a
	// real card an arbitrary amount, off-session, with nobody watching). Same
	// [min,max] as the token top-up — one bound, one definition.
	minCents, maxCents := topupBounds()
	if amountCents < minCents || amountCents > maxCents {
		return receipt{}, fmt.Errorf("%w: %d not in [%d,%d]", errAmountOutOfBounds, amountCents, minCents, maxCents)
	}

	// Idempotency guard (money-critical). This path charges a SAVED card, which —
	// unlike a single-use nonce — is reusable, so there is no natural backstop: a
	// retried request or a re-fired cron would simply charge the card again. The
	// guard is the whole protection, so it is taken BEFORE any money moves and
	// fails CLOSED: if the store is unavailable we cannot tell a first attempt
	// from a retry, and refusing costs a retry while proceeding costs the
	// customer a duplicate charge on their statement.
	rec, replay, gerr := idemBegin(db, "billing-charge:"+userId, idemKey)
	if gerr != nil {
		return receipt{}, fmt.Errorf("%w: %v", errGuardUnavailable, gerr)
	}
	if replay {
		// Already ran to completion: replay the stored answer, do not re-charge.
		if rec.Status == idempotencykey.StatusCompleted && rec.Response != "" {
			var prev receipt
			if json.Unmarshal([]byte(rec.Response), &prev) == nil {
				return prev, nil
			}
		}
		// A genuine concurrent attempt for this key is still in flight — do not
		// run a second money move alongside it.
		return receipt{}, errChargeInFlight
	}
	// abandon releases the guard so a later attempt is not wedged. Called ONLY on
	// failures where no money moved.
	abandon := func() {
		if rec != nil {
			_ = rec.Delete()
		}
	}

	// The card is charged by the processor that holds it (chargeSavedCard), via
	// the shared processorsForOrg seam (production: payment.ProcessorsForOrg) —
	// the SAME one SubscribeWithCard and TopupWithToken use, so a test can
	// substitute the processor for both the saved-card top-up and the cron. The
	// gateway de-dups on a key derived from the SAME stable guard key: the
	// backstop that survives a guard-store outage or two racing submits.
	reg := processorsForOrg(org)
	result, err := chargeSavedCard(ctx, reg, pm, amountCents, cur, gatewayKey("charge", userId, idemKey), description, how)
	if err != nil {
		abandon()
		if _, ok := errors.AsType[*processor.ActionRequired](err); ok {
			return receipt{}, err
		}
		if errors.Is(err, processor.ErrProcessorNotFound) || errors.Is(err, processor.ErrProcessorNotAvailable) || strings.HasPrefix(err.Error(), "no payment processor") {
			return receipt{}, fmt.Errorf("%w: %v", errNoProcessor, err)
		}
		return receipt{}, fmt.Errorf("charge failed: %w", err)
	}
	if !result.Success {
		abandon()
		msg := result.ErrorMessage
		if msg == "" {
			msg = "charge declined"
		}
		return receipt{}, errors.New(msg)
	}

	id, balanceCents, err := fund(ctx, ev, org, db, userId, amountCents, cur, kindOf(pm), result.ProcessorRef)
	if err != nil {
		return receipt{}, err
	}
	r := receipt{TransactionId: id, BalanceCents: int64(balanceCents), ProcessorRef: result.ProcessorRef, AmountCents: amountCents}
	seal(rec, r)
	return r, nil
}

// fund puts money a card has just paid into subject's prepaid balance: the one
// write behind a saved-card top-up, an automatic reload and a sale's starting
// balance. via is the processor that took the payment and ref its reference for
// it, which keys the credit so a retried charge credits once.
//
// It answers the receipt — the deposit row, or the chain mint where credit is
// minted on chain — and the balance read back from the key just credited. A
// credit that fails is money taken and not credited: it raises the money alarm
// and answers errChargedButCreditFailed, and the caller leaves its retry guard
// STARTED so a retry is refused rather than charged again.
func fund(ctx context.Context, ev *events.Client, org *organization.Organization, db *datastore.Datastore, subject string, cents int64, cur currency.Type, via processor.ProcessorType, ref string) (string, currency.Cents, error) {
	// Ledger test-ness MUST follow the charge environment (org.TestMode), not
	// org.Live alone — otherwise a Square sandbox charge could credit the live
	// (spendable) balance. test==credit-bucket==read-bucket==charge-env.
	test := org.TestMode()
	id := ""
	if chainCreditEnabled() {
		// The card was charged (money in), so mint the credit ON CHAIN as prepaid
		// HUSD to the org's derived address instead of a DB deposit row —
		// idempotent by the processor reference so a retried charge credits once.
		rc, err := chainMintCredit(ctx, org, subject, cents, treasury.BucketPrepaid,
			fmt.Sprintf("topup via %s (ref: %s)", via, ref), "topup:"+ref)
		if err != nil {
			uncredited(ctx, ev, org.Name, subject, ref, "the charge settled and the chain mint failed: "+err.Error(), cents, false)
			return "", 0, fmt.Errorf("%w: ref=%s: %v", errChargedButCreditFailed, ref, err)
		}
		id = rc.TxHash
	} else if held, ok, err := deposited(db, subject, via, ref); err != nil {
		// Whether the payment is credited cannot be told, so it is not credited
		// again: the money is owed, and a retry asks again.
		uncredited(ctx, ev, org.Name, subject, ref, "the charge settled and its deposits could not be read: "+err.Error(), cents, false)
		return "", 0, fmt.Errorf("%w: ref=%s: %v", errChargedButCreditFailed, ref, err)
	} else if ok {
		// The payment is already credited: a retry that completes a sale's credit
		// finds the deposit the first attempt wrote, and credits nothing twice.
		id = held
	} else if id, err = lodge(db, subject, cents, cur, via, ref, test); err != nil {
		uncredited(ctx, ev, org.Name, subject, ref, "the charge settled and the deposit failed: "+err.Error(), cents, false)
		return "", 0, fmt.Errorf("%w: ref=%s: %v", errChargedButCreditFailed, ref, err)
	}

	// Read back the new balance so the caller doesn't need a separate request.
	var balanceCents currency.Cents
	if datas, err := util.GetTransactionsByCurrency(org.Namespaced(ctx), subject, "iam-user", cur, test); err == nil {
		if data, ok := datas.Data[cur]; ok {
			balanceCents = data.Balance
		}
	}
	return id, balanceCents, nil
}

// lodge writes the one deposit payment ref makes to subject, atomically: a guard
// keyed on the payment is taken first, an insert only one writer wins, so two
// writers that both found no deposit — a retry crossing its original, two holders
// of a lease that lapsed — write one between them, and the other answers it. A
// deposit written before its guard was sealed is found (deposited) and sealed.
func lodge(db *datastore.Datastore, subject string, cents int64, cur currency.Type, via processor.ProcessorType, ref string, test bool) (string, error) {
	g, replay, err := idempotencykey.Begin(db, "billing-deposit:"+subject, string(via)+":"+ref)
	switch {
	case err != nil:
		return "", err
	case replay && g.Status == idempotencykey.StatusCompleted && g.Response != "":
		return g.Response, nil
	case replay:
		return "", fmt.Errorf("the deposit of payment %s is being written", ref)
	}
	id, ok, err := deposited(db, subject, via, ref)
	if err == nil && !ok {
		trans := transaction.New(db)
		trans.Type = transaction.Deposit
		trans.DestinationId = subject
		trans.DestinationKind = "iam-user"
		trans.Currency = cur
		trans.Amount = currency.Cents(cents)
		trans.Notes = fmt.Sprintf("Top-up via %s (ref: %s)", via, ref)
		trans.Tags = "topup"
		// The processor's own reference for this charge, so the provider's later
		// callback recognises a payment this endpoint already took — the same stamp
		// the token endpoint writes, for the same reason (payment_core.go).
		trans.SourceKind = chargeSourceKind(via)
		trans.SourceId = ref
		trans.Test = test
		// The card was charged: a settled payment IS the mint authority, so
		// authorize THIS write at the ledger sink (money-in == credit).
		trans.SetContext(mintauth.WithAuthorized(trans.Context()))
		err, id = trans.Create(), trans.Id()
	}
	if err != nil {
		// Nothing was written: the guard is released for the retry.
		_ = g.Delete()
		return "", err
	}
	// Sealed with the deposit; a seal that fails leaves the guard to be re-claimed,
	// and the re-claim finds the deposit.
	_ = idempotencykey.Complete(g, id)
	return id, nil
}

// deposited is the deposit a payment already credited subject with, by the
// processor's reference fund stamps on it: one payment is one deposit. A read that
// fails is an error, never "none": answering none would credit the payment again.
func deposited(db *datastore.Datastore, subject string, via processor.ProcessorType, ref string) (string, bool, error) {
	if ref == "" {
		return "", false, nil
	}
	held := transaction.New(db)
	ok, err := held.Query().Filter("SourceKind=", chargeSourceKind(via)).Filter("SourceId=", ref).
		Filter("DestinationId=", subject).Filter("Type=", transaction.Deposit).Get()
	if err != nil || !ok {
		return "", false, err
	}
	return held.Id(), true, nil
}

// seal records the successful charge's answer on its idempotency guard so a
// retry with the same key replays it verbatim instead of charging again.
//
// Note what does NOT call this: the charge-succeeded-but-credit-failed paths
// deliberately leave the guard STARTED, so a same-key retry is refused as
// in-flight rather than re-charging a card whose money already moved. That
// wedges the key until it ages out, which is the correct trade — the failure
// needs reconciliation, not another charge.
func seal(rec *idempotencykey.IdempotencyKey, r receipt) {
	if rec == nil {
		return
	}
	if body, err := json.Marshal(r); err == nil {
		_ = idempotencykey.Complete(rec, string(body))
	}
}

// TopupCardIn is the whole input of a saved-card top-up. Every field is a VALUE
// the caller resolved — nothing is read from a request, a header or the
// environment inside the core.
type TopupCardIn struct {
	// MethodID names the saved card to charge. It must belong to Subject; a
	// method that does not is refused as a miss, never as a "not yours".
	MethodID string
	// AmountCents is the charge in whole cents, bounded server-side by
	// topupBounds before any money moves.
	AmountCents int64
	// Currency is the ISO code. Empty means usd.
	Currency string
	// Subject is the billing key to credit, ALREADY resolved from the caller's
	// own signed identity by the endpoint. The core trusts it and cannot re-derive
	// it: a subject the core chose would be a subject nobody proved.
	Subject string
	// IdempotencyKey is the caller's explicit retry key. Empty falls back to the
	// same windowed derivation over the stable facts — the saved card, the amount
	// and the currency — that a browser sending no header has always used.
	IdempotencyKey string
	// Description rides on the charge to the processor, which is where the
	// customer reads it. The two callers say different true things there ("Top-up"
	// versus "Auto-recharge"), so it is a value rather than a sentence invented here.
	Description string
	// PaymentIntent is the Stripe PaymentIntent (pi_…) a requires_action answer
	// to this same top-up named, which the buyer has since authenticated. The
	// request is sent again whole, with the same key.
	PaymentIntent string
	// Events is the analytics collector the money alarm fires on when a settled
	// charge fails to move the ledger. Nil is a no-op.
	Events *events.Client
}

// TopupCardOut is the settled result — the same three fields the browser has
// always received, with the same names, so the endpoint renders it directly and a
// typed caller reads the same answer.
type TopupCardOut struct {
	// TransactionID is the ledger deposit's id: the receipt for the credit.
	TransactionID string `json:"transactionId"`
	// BalanceCents is the subject's balance read back AFTER the credit, from the
	// same key just credited, so what is returned is what me/balance will report.
	BalanceCents int64 `json:"balanceCents"`
	// Status is "ok" on a settled charge. A failure is an error, never a status.
	Status string `json:"status"`
}

// TopupCard charges a card the subject already saved and credits their balance,
// exactly once.
//
// It is the saved-card half of the pair whose other half is TakePayment (a
// single-use nonce). Both exist because the two rails differ in their backstop:
// a nonce dies on first use, while a card-on-file id is REUSABLE, so on this
// side the idempotency guard is the whole protection and it fails closed.
//
// It takes values rather than a request so the act is reachable by a peer that
// holds no ledger of its own. What it will NOT do is resolve who is paying:
// Subject arrives already bound to the caller's own identity, because a core
// that resolved identity from its own input would let any endpoint hand it a subject
// nobody proved.
//
// The refusal of a method belonging to another subject stays here rather than at
// the endpoint — it is what stands between a member and the org owner's card, and a
// gate only the browser applied would be no gate at all.
func TopupCard(ctx context.Context, org *organization.Organization, in TopupCardIn) (*TopupCardOut, error) {
	if org == nil {
		return nil, errors.New("topup: no organization")
	}
	if in.Subject == "" {
		return nil, errors.New("topup: no subject")
	}
	if strings.TrimSpace(in.MethodID) == "" {
		return nil, methodValidationError{"paymentMethodId is required"}
	}

	db := datastore.New(org.Namespaced(ctx))
	cur := normalizeCurrency(in.Currency)

	// Load the payment method. It must be the PAYING SUBJECT's own instrument:
	// paymentMethodId is an unpinned field that can name another subject's card
	// inside the org, and the credit lands on the subject — charging someone
	// else's card to fund your own balance is the exact cross-subject move this
	// refuses. A miss, so method ids can't be probed. Stricter on purpose than
	// the read-side callerMayReachBillingSubject: a charge binds to the RESOLVED
	// paying subject, so an org member's fine <org>/<user> subject can never
	// spend the org owner's shared card.
	pm := paymentmethod.New(db)
	if err := pm.GetById(strings.TrimSpace(in.MethodID)); err != nil {
		return nil, fmt.Errorf("%w: %v", errNoMethod, err)
	}
	if pm.CustomerId != in.Subject && pm.UserId != in.Subject {
		return nil, errNoMethod
	}
	if strings.TrimSpace(pm.ProviderRef) == "" {
		return nil, errMethodUnchargeable
	}

	// Stable across a retry: the saved card, the amount and the currency, in a
	// coarse window. Never the payment-method row's mutable state, never a nonce.
	// ONE derivation, here, so the browser's fallback and a peer's cannot drift
	// into disagreeing about what "the same request" means.
	idemKey := in.IdempotencyKey
	if idemKey == "" {
		idemKey = windowKey("pm:" + strings.TrimSpace(in.MethodID) + ":amount:" + strconv.FormatInt(in.AmountCents, 10) + ":cur:" + string(cur))
	}

	desc := in.Description
	if desc == "" {
		desc = fmt.Sprintf("Top-up %d %s for %s", in.AmountCents, cur, in.Subject)
	}

	// A top-up is a new payment: only a card of the card rail makes one. A card
	// saved at a rail that closed keeps paying what it already pays.
	if !spends(ctx, pm) {
		return nil, errCardReadd
	}
	r, err := chargeAndCredit(ctx, in.Events, org, db, pm, in.AmountCents, cur, in.Subject, idemKey, desc,
		cardCharge{resume: strings.TrimSpace(in.PaymentIntent)})
	if err != nil {
		return nil, err
	}
	// The payment's wallet, as the token top-up records it (payment_core.go), so a
	// refund or dispute of the payment takes the money back from the wallet it funded.
	if r.ProcessorRef != "" {
		if err := paymentorg.Put(r.ProcessorRef, org.Name, in.Subject, "", 0, org.TestMode()); err != nil {
			log.Warn("topup: payment %s: the index to its wallet was not written: %v", r.ProcessorRef, err)
		}
	}
	return &TopupCardOut{TransactionID: r.TransactionId, BalanceCents: r.BalanceCents, Status: "ok"}, nil
}

// Topup charges a saved payment method and credits the user's balance.
//
//	POST /v1/billing/topup
//
// Body: { userId, paymentMethodId, amountCents, currency? }
// Returns: { transactionId, balanceCents, status }
//
// Everything below the bind is reading values off the request and handing them
// to TopupCard: the payer from the caller's signed identity, the retry key from
// the header, the analytics client from the request locals. The status
// each failure carries is the endpoint's to decide, which is why the core answers
// with a class and this maps it.
func Topup(c *zip.Ctx) error {
	// The OK form: IAMTokenRequired falls through WITHOUT setting the
	// "organization" local when the gateway named no principal, and the MustGet
	// form panics there — a 500 with no body, after the money has moved. Refuse
	// before touching anything. See SubscribeWithCard, where this cost a $99
	// charge with no subscription behind it.
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return jsonhttp.Fail(c, 401, "sign in to add funds", nil)
	}
	// A card on file is the wallet's: charging the org's is its admins' act.
	if !mayChangeWallet(c) {
		return jsonhttp.Fail(c, 403, askAnAdmin(c, "charge its card on file"), nil)
	}

	var req topupRequest
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}

	// Credit the payer resolved from the caller's signed identity (userBillingKey
	// → account.Payer) — the ONE key the LLM gate debits and GetMyBalance reads.
	// A request-supplied userId must NOT steer the destination; a client-set
	// selector is exactly what split a customer's top-ups across `hanzo` and
	// `hanzo/<user>`, stranding credit off the key their usage draws from.
	subject := userBillingKey(c)
	if subject == "" {
		return jsonhttp.Fail(c, 401, "missing identity headers", nil)
	}

	out, err := TopupCard(c.Context(), org, TopupCardIn{
		MethodID:       req.PaymentMethodID,
		AmountCents:    req.AmountCents,
		Currency:       req.Currency,
		Subject:        subject,
		IdempotencyKey: strings.TrimSpace(c.Header("X-Idempotency-Key")),
		Events:         eventsOf(c),
	})
	if err != nil {
		switch {
		case IsMethodRefused(err):
			return jsonhttp.Fail(c, 400, err.Error(), nil)
		case IsMethodNotFound(err):
			return jsonhttp.Fail(c, 404, "payment method not found", err)
		case IsMethodUnchargeable(err):
			return jsonhttp.Fail(c, 422, "saved payment method has no chargeable card — add the card again", nil)
		case IsTopupOutOfBounds(err):
			minCents, maxCents := topupBounds()
			return jsonhttp.Fail(c, 400, fmt.Sprintf("amountCents must be between %d and %d", minCents, maxCents), nil)
		case IsTopupGuardUnavailable(err):
			log.Error("topup idempotency guard unavailable (subject=%s): %v", subject, err, c)
			return jsonhttp.Fail(c, 503, "billing is temporarily unavailable; please retry", err)
		case IsTopupInFlight(err):
			return jsonhttp.Fail(c, 409, "top-up already in progress", nil)
		case IsTopupNoProcessor(err):
			log.Error("No processor available for topup: %v", err, c)
			return jsonhttp.Fail(c, 422, "no payment processor available", err)
		case IsTopupUncredited(err):
			return jsonhttp.Fail(c, 500, "charge succeeded but balance credit failed; contact support", err)
		default:
			log.Error("Charge failed for topup (subject=%s pm=%s): %v", subject, req.PaymentMethodID, err, c)
			return jsonhttp.Fail(c, 402, err.Error(), nil)
		}
	}

	return c.JSON(200, out)
}
