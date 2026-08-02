package billing

import (
	"encoding/json"
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
	// Idem makes a retried payout return the FIRST payout's response rather
	// than creating a second one. It is the caller's key; X-Idempotency-Key is
	// the same key by another door, and with neither the request de-dups
	// against the coarse window every card money move in this package uses (see
	// idem.go) so a lost-response retry cannot pay twice.
	Idem string `json:"idem,omitempty"`
}

// CreatePayout creates a new outbound payout.
//
//	POST /v1/billing/payouts
//
// IT IS IDEMPOTENT, and the guard is taken BEFORE anything happens. A payout is
// money leaving with no natural backstop — no nonce is consumed, no card
// refuses the second charge — so a retried request, a double-clicked console,
// or a client that resends because it never saw the response is a SECOND
// PAYOUT unless something says otherwise. The row's own doc claimed the Idem
// field made a retry return the first answer; nothing implemented that, so the
// promise was a lie of exactly the kind that gets believed on a money route.
//
// It fails CLOSED. If the guard store cannot tell a first attempt from a retry,
// the payout is refused: that costs the caller a retry, while proceeding costs
// the merchant a duplicate disbursement.
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

	guard, replay, gerr := idemBegin(db, payoutScope+req.DestinationId, payoutKey(c, req))
	if gerr != nil {
		log.Error("Failed to take the payout idempotency guard: %v", gerr, c)
		return http.Fail(c, 503, "payout guard unavailable; retry", gerr)
	}
	if replay {
		// Ran to completion already: give back the FIRST payout's answer,
		// verbatim, and create nothing.
		if guard.Status == idempotencykey.StatusCompleted && guard.Response != "" {
			var prior map[string]interface{}
			if json.Unmarshal([]byte(guard.Response), &prior) == nil {
				return c.JSON(201, prior)
			}
		}
		// A genuine attempt under this key is still in flight. Do not run a
		// second payout alongside it.
		return http.Fail(c, 409, "a payout under this idempotency key is already in flight", nil)
	}
	// abandon releases the guard when NO money was committed, so a refusal does
	// not wedge the key: the caller may fix the cause and ask again, and the
	// next attempt is screened against the controls in force then rather than
	// replaying a refusal from a restraint that has since been lifted.
	abandon := func() {
		if guard != nil {
			_ = guard.Delete()
		}
	}

	// The money plane's own gate. A reserve or a payout hold in force on this
	// merchant stops the money HERE, in the store that holds the control — no
	// network hop, so a scoring outage can never lift a restraint. Risk
	// declares; this is where commerce enforces.
	gate, err := gatePayout(c, db, req)
	if err != nil {
		abandon()
		log.Error("Failed to screen payout: %v", err, c)
		return http.Fail(c, 500, "failed to screen the payout", err)
	}
	if gate.Status != 0 {
		abandon()
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
		abandon()
		log.Error("Failed to create payout: %v", err, c)
		return http.Fail(c, 500, "failed to create payout", err)
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

	// Seal the guard with the answer, so a retry replays THIS response instead
	// of paying again. A seal that fails leaves the guard "started", which
	// refuses the retry as in-flight — the right way round: a payout that
	// already happened must never be repeated because its receipt did not save.
	if body, jerr := json.Marshal(resp); jerr == nil {
		if serr := idempotencykey.Complete(guard, string(body)); serr != nil {
			log.Error("Failed to seal the payout idempotency guard: %v", serr, c)
		}
	}
	return c.JSON(201, resp)
}

// payoutScope namespaces a payout's idempotency key to its destination, so one
// key can never collide across endpoints or across the accounts it might pay.
const payoutScope = "billing-payout:"

// payoutKey is the key a payout guards itself on: the caller's Idem, else the
// X-Idempotency-Key header, else a key derived from the facts that stay STABLE
// across a retry, bucketed into the coarse window every card money move in this
// package already uses.
//
// The window is what makes the fallback safe in both directions: a re-submit
// seconds later collapses onto the same key, so the money leaves once; a
// genuine second payout to the same account minutes later gets a fresh key and
// proceeds. A merchant that needs two identical payouts inside the window says
// so by sending two different keys — which is what a key is for.
func payoutKey(c *zip.Ctx, req createPayoutRequest) string {
	if k := strings.TrimSpace(req.Idem); k != "" {
		return k
	}
	return guardKey(c, strings.Join([]string{
		"payout", req.DestinationType, req.DestinationId, req.Merchant,
		req.Currency, strconv.FormatInt(req.Amount, 10),
	}, ":"))
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

	// Bounded at the store. A payout list grows with every disbursement an org
	// ever made, so a read with no ceiling is one request that materialises the
	// whole history — in a process shared with every other tenant.
	rootKey := db.NewKey("synckey", "", 1, nil)
	payouts := make([]*payout.Payout, 0)
	iter := payout.Query(db).Ancestor(rootKey).Order("-CreatedAt").Limit(pageMax).Run()

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
func gatePayout(c *zip.Ctx, db *datastore.Datastore, req createPayoutRequest) (payoutGate, error) {
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
		Idem:      req.Idem,
	})
	if err != nil {
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
		Allow:  currency.Cents(rec.Allowed),
		Held:   currency.Cents(rec.Held),
		Screen: rec.Id(),
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
