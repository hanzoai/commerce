package billing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/payout"
	"github.com/hanzoai/commerce/models/screen"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/risk"
	"github.com/hanzoai/commerce/util/json/http"
)

type createPayoutRequest struct {
	Amount          int64                  `json:"amount"`
	Currency        string                 `json:"currency,omitempty"`
	DestinationType string                 `json:"destinationType"` // "bank_account" | "card"
	DestinationId   string                 `json:"destinationId"`
	Description     string                 `json:"description,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`

	// Merchant names whose money this is, on a platform. It is what a reserve
	// or a payout hold is placed on, so a payout that names one is restrained
	// by that merchant's controls; one that does not is restrained by the
	// destination's.
	Merchant string `json:"merchant,omitempty"`
	// Idem makes a retried payout return the first answer rather than a second
	// judgement — and, far more importantly, than a second payout. A caller may
	// send it here or as X-Idempotency-Key; with neither, retries within a
	// coarse window still collapse onto one payout by their own facts (see
	// guardKey), because a retried payout is the ordinary case and a double
	// payout is unrecoverable.
	Idem string `json:"idem,omitempty"`
}

// facts are the payout details that stay STABLE across a retry, in one
// deterministic string. They are the fallback idempotency key for a caller that
// sent none, and the fingerprint that stops one key from naming two different
// payouts.
func (r createPayoutRequest) facts() string {
	return strings.Join([]string{
		strconv.FormatInt(r.Amount, 10),
		strings.ToLower(r.Currency),
		r.DestinationType,
		r.DestinationId,
		r.Merchant,
	}, "|")
}

func (r createPayoutRequest) digest() string {
	sum := sha256.Sum256([]byte(r.facts()))
	return hex.EncodeToString(sum[:16])
}

// CreatePayout creates a new outbound payout.
//
// IT IS IDEMPOTENT, through the same guard every other money move in this
// package uses (models/idempotencykey): a retry replays the FIRST payout's
// exact response and creates no second row. The doc used to promise this and
// the code did not implement it — the key reached the risk screen, which
// de-duplicated the JUDGEMENT, while `payout.Create` ran again underneath it
// and the merchant was paid twice. An idempotency key that lies is worse than
// none: a client retries a timeout precisely because it was promised the retry
// is free.
//
// A key naming a DIFFERENT payout is refused (409) rather than answered with
// the first one's response — see [createPayoutRequest.digest].
//
//	POST /v1/billing/payouts
func CreatePayout(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	var req createPayoutRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	if req.Amount <= 0 {
		return http.Fail(c, 400, "amount must be positive", nil)
	}
	if req.DestinationId == "" {
		return http.Fail(c, 400, "destinationId is required", nil)
	}

	// The guard comes FIRST, before the screen and before the row. Scoped to
	// the destination so a key can never collide across endpoints, keyed on the
	// caller's own key when there is one and on the payout's stable facts when
	// there is not, and fingerprinted so one key names one payout. Fail CLOSED
	// (503) when the guard store cannot answer: a payout we cannot de-duplicate
	// must not run.
	key := req.Idem
	if key == "" {
		key = guardKey(c, req.facts())
	}
	guard, replay, gerr := idemBegin(db, "billing-payout:"+req.DestinationId, key, req.digest())
	switch {
	case errors.Is(gerr, idempotencykey.ErrDigest):
		return http.Fail(c, 409, "this idempotency key already names a different payout", gerr)
	case gerr != nil:
		log.Error("payout idempotency Begin failed (destination=%s): %v", req.DestinationId, gerr, c)
		return http.Fail(c, 503, "payout temporarily unavailable; retry", gerr)
	case replay:
		if guard.Status == idempotencykey.StatusCompleted && guard.Response != "" {
			c.SetHeader("Content-Type", "application/json")
			return c.Bytes(200, []byte(guard.Response))
		}
		return http.Fail(c, 409, "a payout for this request is already in progress", nil)
	}

	// The money plane's own gate. A reserve or a payout hold in force on this
	// merchant stops the money HERE, in the store that holds the control — no
	// network hop, so a scoring outage can never lift a restraint. Risk
	// declares; this is where commerce enforces.
	gate, err := gatePayout(c, db, req, key)
	if err != nil {
		_ = guard.Delete()
		log.Error("Failed to screen payout: %v", err, c)
		return http.Fail(c, 500, "failed to screen the payout", err)
	}
	if gate.Status != 0 {
		// Nothing moved, so the guard is released: a refusal is not an answer
		// to replay, and a merchant whose hold is lifted must be able to ask
		// again under the same key.
		_ = guard.Delete()
		return http.Fail(c, gate.Status, gate.Message, nil)
	}

	p := payout.New(db)
	p.Amount = int64(gate.Allow)
	if req.Currency != "" {
		p.Currency = currency.Type(req.Currency)
	}
	p.DestinationType = req.DestinationType
	p.DestinationId = req.DestinationId
	p.Description = req.Description
	if req.Metadata != nil {
		p.Metadata = req.Metadata
	}

	if err := p.Create(); err != nil {
		_ = guard.Delete()
		log.Error("Failed to create payout: %v", err, c)
		return http.Fail(c, 500, "failed to create payout", err)
	}

	// The money did not leave, so the ledger records that it is being HELD.
	// Posted here and not at the screen: a judgement is a question, and asking
	// it a thousand times must not withhold a thousand shares. Idempotent on
	// the screen row, which a retry replays.
	if _, err := gate.Screener.Hold(gate.Record, "payout "+p.Id()); err != nil {
		log.Error("Failed to post the reserve hold for payout %s: %v", p.Id(), err, c)
	}

	// A reserve is DISCLOSED, never silent: the response states what was asked
	// for, what was withheld and the screen that decided it, so a merchant
	// reconciling a short payout can see why without asking.
	resp := payoutResponse(p)
	resp["screen"] = gate.Screen
	if gate.Held > 0 {
		resp["requested"] = req.Amount
		resp["held"] = int64(gate.Held)
	}
	// Seal the guard with the exact body, so a retry replays this payout rather
	// than creating another one.
	if body, mErr := json.Marshal(resp); mErr == nil {
		_ = idempotencykey.Complete(guard, string(body))
	}
	return c.JSON(201, resp)
}

// GetPayout retrieves a payout by ID.
//
//	GET /v1/billing/payouts/:id
func GetPayout(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	p := payout.New(db)
	if err := p.GetById(c.Param("id")); err != nil {
		return http.Fail(c, 404, "payout not found", err)
	}

	return c.JSON(200, payoutResponse(p))
}

// ListPayouts lists payouts.
//
//	GET /v1/billing/payouts
func ListPayouts(c *zip.Ctx) error {
	// #146 class: never panic on a missing org (co-resident embed path — see ListInvoices).
	// No org ⇒ honest empty list.
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return c.JSON(200, []map[string]interface{}{})
	}
	db := datastore.New(org.Namespaced(c.Context()))

	rootKey := db.NewKey("synckey", "", 1, nil)
	payouts := make([]*payout.Payout, 0)
	iter := payout.Query(db).Ancestor(rootKey).Order("-Created").Run()

	for {
		p := payout.New(db)
		if _, err := iter.Next(p); err != nil {
			break
		}
		payouts = append(payouts, p)
	}

	results := make([]map[string]interface{}, len(payouts))
	for i, p := range payouts {
		results[i] = payoutResponse(p)
	}
	return c.JSON(200, results)
}

// CancelPayout cancels a pending payout.
//
//	POST /v1/billing/payouts/:id/cancel
func CancelPayout(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	p := payout.New(db)
	if err := p.GetById(c.Param("id")); err != nil {
		return http.Fail(c, 404, "payout not found", err)
	}

	if err := p.Cancel(); err != nil {
		return http.Fail(c, 400, err.Error(), err)
	}

	if err := p.Update(); err != nil {
		log.Error("Failed to cancel payout: %v", err, c)
		return http.Fail(c, 500, "failed to cancel payout", err)
	}

	return c.JSON(200, payoutResponse(p))
}

// payoutGate is what the controls say about one payout: how much may actually
// leave, how much a reserve withheld, and — when nothing may leave — the status
// and message the caller gets.
//
// It is a VALUE and not a written response because http.Fail writes the body
// and returns nil — so a gate that "returned the refusal" would be a refusal
// the caller checked with `err != nil` and never saw, and the payout would be
// created after its own refusal had already been written. One writer, one
// judgement, no way to hold it wrong.
type payoutGate struct {
	Status  int
	Message string
	// Allow and Held are exact minor units and always sum to the requested
	// amount.
	Allow  currency.Cents
	Held   currency.Cents
	Screen string
	// Record and Screener are what the caller needs to post the withheld share
	// to the reserve ledger once the payout row actually exists. They are
	// carried rather than re-derived so the ledger cites the SAME screen the
	// response quotes.
	Record   *screen.Screen
	Screener *risk.Screener
}

// gatePayout screens the payout before a row is written and reports how much
// may leave. The error return is a FAULT (the screen could not be taken), never
// a judgement.
//
// A hold or a block REFUSES; a reserve WITHHOLDS ITS SHARE and lets the rest
// go. Refusing a reserved payout outright reads well until you try to satisfy
// the refusal: a reserve applies to whatever is asked for, so "ask for less"
// simply reserves a share of the smaller amount too, and the caller is walked
// in a circle it can never leave. Withholding the share is what a reserve IS —
// and it is disclosed in the response, so it is not a silent shrink.
func gatePayout(c *zip.Ctx, db *datastore.Datastore, req createPayoutRequest, key string) (payoutGate, error) {
	subject := risk.Subject{Kind: risk.KindPayout, ID: req.DestinationId}
	if req.Merchant != "" {
		subject = risk.Subject{Kind: risk.KindMerchant, ID: req.Merchant}
	}

	s := &risk.Screener{DB: db, By: whoever(c)}
	rec, err := s.Screen(c.Context(), risk.Move{
		Stage:     risk.Payout,
		Subject:   subject,
		Amount:    currency.Cents(req.Amount),
		Currency:  currency.Type(req.Currency),
		Out:       true,
		Reference: req.DestinationId,
		Idem:      key,
	})
	switch {
	case errors.Is(err, risk.ErrReused):
		return payoutGate{Status: 409, Message: err.Error()}, nil
	case err != nil:
		return payoutGate{}, err
	}
	if risk.Refused(rec) {
		return payoutGate{Status: 403, Message: payoutRefusal(rec), Screen: rec.Id()}, nil
	}
	if rec.Allowed <= 0 {
		// A full reserve leaves nothing to send. Creating a payout of zero is
		// not a payout.
		return payoutGate{Status: 403, Message: fmt.Sprintf(
			"a reserve withholds all %d of this payout", req.Amount), Screen: rec.Id()}, nil
	}
	return payoutGate{
		Allow:    currency.Cents(rec.Allowed),
		Held:     currency.Cents(rec.Held),
		Screen:   rec.Id(),
		Record:   rec,
		Screener: s,
	}, nil
}

func payoutRefusal(rec *screen.Screen) string {
	if rec.Reason == "" {
		return "payout refused by risk"
	}
	return "payout refused by risk: " + rec.Reason
}

// whoever is the validated principal, or empty. It never panics on a request
// that carries no user: a platform-token payout has an org and no person, and
// an unattributed record is honest where a panic is not.
func whoever(c *zip.Ctx) string {
	if u, ok := c.Locals("user").(*user.User); ok && u != nil {
		return u.Id()
	}
	return ""
}

func payoutResponse(p *payout.Payout) map[string]interface{} {
	resp := map[string]interface{}{
		"id":              p.Id(),
		"amount":          p.Amount,
		"currency":        p.Currency,
		"status":          p.Status,
		"destinationType": p.DestinationType,
		"destinationId":   p.DestinationId,
		// GetCreatedAt, not Created: Payout declares no Created FIELD, so
		// `p.Created` was the mixin's `Created() bool` METHOD, and marshalling a
		// func value fails — every successful create and every non-empty list
		// answered 500 "json: unsupported type: func() bool". A payout row was
		// written and the caller was told the request had failed.
		"created": p.GetCreatedAt(),
	}
	if p.Description != "" {
		resp["description"] = p.Description
	}
	if !p.ArrivalDate.IsZero() {
		resp["arrivalDate"] = p.ArrivalDate
	}
	if p.ProviderRef != "" {
		resp["providerRef"] = p.ProviderRef
	}
	if p.FailureCode != "" {
		resp["failureCode"] = p.FailureCode
		resp["failureMessage"] = p.FailureMessage
	}
	if p.Metadata != nil {
		resp["metadata"] = p.Metadata
	}
	return resp
}
