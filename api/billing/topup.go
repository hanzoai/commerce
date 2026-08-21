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
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/thirdparty/kms"
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

// The classes a top-up can fail in, published so a caller outside this package
// answers them the way the door answers them. Each is a different thing for the
// asker to do — fix the request, retry later, use another card, call support —
// which is why they are separate questions and not one "it failed".
//
// Two of the classes are the package's and not this file's: a request naming no
// card is IsMethodRefused, and a card that is not the paying subject's is
// IsMethodNotFound. Reusing them is what keeps "not there, or not yours" a
// single answer everywhere, which is the whole reason it cannot be probed.

// IsTopupOutOfBounds reports whether the amount sits outside the server-side
// [min,max]. The bounds are the door's to state, since it is the door that
// knows how to say them; this only reports that they were crossed.
func IsTopupOutOfBounds(err error) bool { return errors.Is(err, errAmountOutOfBounds) }

// IsMethodUnchargeable reports whether the saved method carries no chargeable
// card. It is the package's answer rather than this file's: the card subscribe
// path refuses the same row for the same reason and says the same sentence, and
// two names for one condition is how two doors come to disagree about it.
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
type receipt struct {
	TransactionId string `json:"transactionId"`
	BalanceCents  int64  `json:"balanceCents"`
}

type topupRequest struct {
	// No userId: the credited subject is the caller's own signed identity
	// (userBillingKey → account.Payer), never a body field, so a request can not
	// steer where the money lands. Same rule as the token door.
	PaymentMethodID string `json:"paymentMethodId"`
	AmountCents     int64  `json:"amountCents"`
	Currency        string `json:"currency,omitempty"`
}

// chargeAndCredit charges a saved payment method via the org's KMS-hydrated
// processor registry and, on success, credits the user's prepaid balance with a
// Deposit transaction. The org MUST already be KMS-hydrated by the caller (so
// payment.ProcessorsForOrg sees real Square credentials). Returns the deposit
// transaction id and the new balance.
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
func chargeAndCredit(ctx context.Context, ev *events.Client, org *organization.Organization, db *datastore.Datastore, pm *paymentmethod.PaymentMethod, amountCents int64, cur currency.Type, userId, idemKey, description string) (string, currency.Cents, error) {
	// Server-authoritative amount bounds, enforced HERE so both callers inherit
	// them: the HTTP top-up (where a scripted request bypasses the browser cap)
	// and the auto-recharge cron (where a mis-set config would otherwise charge a
	// real card an arbitrary amount, off-session, with nobody watching). Same
	// [min,max] as the token top-up — one bound, one definition.
	minCents, maxCents := topupBounds()
	if amountCents < minCents || amountCents > maxCents {
		return "", 0, fmt.Errorf("%w: %d not in [%d,%d]", errAmountOutOfBounds, amountCents, minCents, maxCents)
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
		return "", 0, fmt.Errorf("%w: %v", errGuardUnavailable, gerr)
	}
	if replay {
		// Already ran to completion: replay the stored answer, do not re-charge.
		if rec.Status == idempotencykey.StatusCompleted && rec.Response != "" {
			var prev receipt
			if json.Unmarshal([]byte(rec.Response), &prev) == nil {
				return prev.TransactionId, currency.Cents(prev.BalanceCents), nil
			}
		}
		// A genuine concurrent attempt for this key is still in flight — do not
		// run a second money move alongside it.
		return "", 0, errChargeInFlight
	}
	// abandon releases the guard so a later attempt is not wedged. Called ONLY on
	// failures where no money moved.
	abandon := func() {
		if rec != nil {
			_ = rec.Delete()
		}
	}

	squareCustomerID := pm.CustomerId
	if pm.Metadata != nil {
		if v, ok := pm.Metadata["squareCustomerId"].(string); ok && v != "" {
			squareCustomerID = v
		}
	}

	chargeReq := processor.PaymentRequest{
		Token:      pm.ProviderRef,
		Amount:     currency.Cents(amountCents),
		Currency:   cur,
		CustomerID: squareCustomerID,
		// The gateway de-dups on this key, derived from the SAME stable guard key.
		// It is the backstop that survives a guard-store outage or two racing
		// submits, because it is enforced where the money actually moves.
		IdempotencyKey: gatewayKey("charge", userId, idemKey),
		Description:    description,
	}

	// The global registry holds empty singletons (Square is registered
	// unconfigured at init), so charges must go through the org-scoped registry.
	//
	// Via the shared processorsForOrg seam (production: payment.ProcessorsForOrg)
	// — the SAME one SubscribeWithCard and TopupWithToken use. Reaching for
	// payment.ProcessorsForOrg directly is what made this primitive untestable:
	// no test could substitute a processor, so the saved-card top-up AND the
	// off-session auto-recharge cron both ran with zero coverage.
	reg := processorsForOrg(org)
	proc, err := reg.SelectProcessor(ctx, chargeReq)
	if err != nil {
		abandon()
		return "", 0, fmt.Errorf("%w: %v", errNoProcessor, err)
	}

	result, err := proc.Charge(ctx, chargeReq)
	if err != nil {
		abandon()
		return "", 0, fmt.Errorf("charge failed: %w", err)
	}
	if !result.Success {
		abandon()
		msg := result.ErrorMessage
		if msg == "" {
			msg = "charge declined"
		}
		return "", 0, errors.New(msg)
	}

	// Step 4: the card was charged (money in), so mint the credit ON CHAIN as
	// prepaid HUSD to the org's derived address instead of a DB deposit row —
	// idempotent by the processor reference so a retried charge credits once.
	if chainCreditEnabled() {
		rc, mErr := chainMintCredit(ctx, org, userId, amountCents, treasury.BucketPrepaid,
			fmt.Sprintf("topup via %s (ref: %s)", proc.Type(), result.ProcessorRef),
			"topup:"+result.ProcessorRef)
		if mErr != nil {
			uncredited(ctx, ev, org.Name, userId, result.ProcessorRef,
				"the charge settled and the chain mint failed: "+mErr.Error(),
				int64(amountCents), false)
			return "", 0, fmt.Errorf("%w: ref=%s: %v", errChargedButCreditFailed, result.ProcessorRef, mErr)
		}
		var balanceCents currency.Cents
		if datas, bErr := util.GetTransactionsByCurrency(org.Namespaced(ctx), userId, "iam-user", cur, org.TestMode()); bErr == nil {
			if data, ok := datas.Data[cur]; ok {
				balanceCents = data.Balance
			}
		}
		seal(rec, rc.TxHash, balanceCents)
		return rc.TxHash, balanceCents, nil
	}

	trans := transaction.New(db)
	trans.Type = transaction.Deposit
	trans.DestinationId = userId
	trans.DestinationKind = "iam-user"
	trans.Currency = cur
	trans.Amount = currency.Cents(amountCents)
	trans.Notes = fmt.Sprintf("Top-up via %s (ref: %s)", proc.Type(), result.ProcessorRef)
	trans.Tags = "topup"
	// The processor's own reference for this charge, so the provider's later callback
	// recognises a payment this door already took — the same stamp the token door
	// writes, for the same reason (payment_core.go).
	trans.SourceKind = chargeSourceKind(proc.Type())
	trans.SourceId = result.ProcessorRef
	// Ledger test-ness MUST follow the charge environment (org.TestMode), not
	// org.Live alone — otherwise a Square sandbox charge could credit the live
	// (spendable) balance. test==credit-bucket==read-bucket==charge-env.
	test := org.TestMode()
	trans.Test = test
	// The card was charged (result.Success): a settled payment IS the mint
	// authority, so authorize THIS write at the ledger sink (money-in == credit).
	trans.SetContext(mintauth.WithAuthorized(trans.Context()))
	if err := trans.Create(); err != nil {
		// Charge succeeded but credit failed — log with full context for manual reconciliation.
		uncredited(ctx, ev, org.Name, userId, result.ProcessorRef,
			"the charge settled and the deposit failed: "+err.Error(),
			int64(amountCents), false)
		return "", 0, fmt.Errorf("%w: ref=%s: %v", errChargedButCreditFailed, result.ProcessorRef, err)
	}

	// Read back the new balance so the caller doesn't need a separate request.
	var balanceCents currency.Cents
	if datas, err := util.GetTransactionsByCurrency(org.Namespaced(ctx), userId, "iam-user", cur, test); err == nil {
		if data, ok := datas.Data[cur]; ok {
			balanceCents = data.Balance
		}
	}
	seal(rec, trans.Id(), balanceCents)
	return trans.Id(), balanceCents, nil
}

// seal records the successful charge's answer on its idempotency guard so a
// retry with the same key replays it verbatim instead of charging again.
//
// Note what does NOT call this: the charge-succeeded-but-credit-failed paths
// deliberately leave the guard STARTED, so a same-key retry is refused as
// in-flight rather than re-charging a card whose money already moved. That
// wedges the key until it ages out, which is the correct trade — the failure
// needs reconciliation, not another charge.
func seal(rec *idempotencykey.IdempotencyKey, txID string, balanceCents currency.Cents) {
	if rec == nil {
		return
	}
	if body, err := json.Marshal(receipt{TransactionId: txID, BalanceCents: int64(balanceCents)}); err == nil {
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
	// own signed identity by the door. The core trusts it and cannot re-derive
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
	// KMS is the request-scoped cached client used to hydrate the org's payment
	// credentials. Nil is legitimate and non-fatal (dev/tests, env-var creds).
	KMS *kms.CachedClient
	// Events is the analytics collector the money alarm fires on when a settled
	// charge fails to move the ledger. Nil is a no-op.
	Events *events.Client
}

// TopupCardOut is the settled result — the same three fields the browser has
// always received, with the same names, so the door renders it directly and a
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
// that resolved identity from its own input would let any door hand it a subject
// nobody proved.
//
// The refusal of a method belonging to another subject stays here rather than at
// the door — it is what stands between a member and the org owner's card, and a
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

	// Best-effort credential hydration, exactly as the browser path did: a
	// missing KMS client is dev/test with env-var creds, not a failure.
	if in.KMS != nil {
		if err := kms.Hydrate(in.KMS, org); err != nil {
			log.Error("KMS hydration failed for org %q: %v", org.Name, err)
		}
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

	txID, balanceCents, err := chargeAndCredit(ctx, in.Events, org, db, pm, in.AmountCents, cur, in.Subject, idemKey, desc)
	if err != nil {
		return nil, err
	}
	return &TopupCardOut{TransactionID: txID, BalanceCents: int64(balanceCents), Status: "ok"}, nil
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
// the header, the KMS and analytics clients from the request locals. The status
// each failure carries is the door's to decide, which is why the core answers
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
		KMS:            kmsOf(c),
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
