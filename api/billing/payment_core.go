// Copyright 2014-present Hanzo AI Inc. Licensed under MIT OR Apache-2.0.

package billing

// The CARD MONEY MOVE, as one core with no HTTP in it.
//
// Taking a payment used to exist only as TopupWithToken, a *zip.Ctx handler. That
// shape is not a limitation of the money logic — it is a limitation of where the
// logic was WRITTEN. A handler that reads its inputs off a request can only ever
// be reached by something holding a request, which meant the one operation that
// turns a card into a balance was unreachable from a typed op, and therefore
// invisible to every projection a typed op feeds: the OpenAPI operation, the MCP
// tool an agent calls, the generated SDK method, the CLI command.
//
// So the core moved here and grew an input instead of a request. TakePayment is
// the WHOLE money move — bounds, idempotency, processor selection, charge, ledger
// credit, balance read-back — and it takes values. TopupWithToken is now a
// projection of it that reads those values off a *zip.Ctx, and the typed op in
// the host is another projection that gets them from a decoded body. There is
// exactly ONE charge path underneath both, which is the only reason it is safe
// to have two doors: a second implementation of "charge a card and credit the
// ledger" is a second set of bounds to drift, a second idempotency derivation to
// disagree, and eventually a double charge nobody can explain.
//
// WHAT THIS CORE DOES NOT DO, on purpose:
//
//   - It does not resolve WHO is paying. Subject arrives already bounded to the
//     caller's org (inOrgSubject), because the bound belongs to the identity
//     boundary and each door has its own: the browser door reads ?user= behind
//     EdgeAuth, the typed door reads the validated tenant off the context. A core
//     that resolved identity from its input would let either door hand it a
//     subject the caller never proved, which is the whole IDOR class.
//   - It does not read the environment for its idempotency key. The key arrives
//     as a value; an empty one falls back to the SAME windowKey derivation the
//     header-less browser path has always used, so the two doors cannot drift
//     into disagreeing about what "the same request" means.
//   - It does not choose sandbox or production. That is the org's KMS-hydrated
//     Square credentials and org.TestMode(), exactly as before — the caller
//     cannot ask for a test charge, because a caller that could would be able to
//     mint spendable balance from a sandbox card.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/mintauth"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/thirdparty/kms"

	"github.com/hanzoai/commerce/datastore"
)

// TakePaymentIn is the whole input of a card money move. Every field is a VALUE
// the caller resolved — nothing is read from a request, an environment variable
// or a header inside the core.
type TakePaymentIn struct {
	// SourceID is the single-use payment token: a Square Web Payments SDK nonce
	// from the browser, or a Square sandbox test nonce. It is the ONLY thing that
	// stands in for a card here — the PAN never reaches this process. A card the
	// subject already SAVED is charged by the other door (Topup → chargeAndCredit),
	// never through this core.
	SourceID string
	// AmountCents is the charge in whole cents. Bounded server-side by
	// topupBounds; an out-of-range amount is refused before any money moves.
	AmountCents int64
	// Currency is the ISO code. Empty means usd.
	Currency string
	// Subject is the billing key to credit, ALREADY bounded to the caller's own
	// org by the door that resolved it (inOrgSubject). The core trusts it and
	// cannot re-derive it — see the package note.
	Subject string
	// IdempotencyKey is the caller's explicit retry key. Empty falls back to the
	// windowed derivation over the stable request facts, which is what the
	// browser path does when it sends no X-Idempotency-Key.
	IdempotencyKey string
	// KMS is the request-scoped cached client used to hydrate the org's payment
	// credentials. Nil is legitimate and non-fatal (dev/tests, env-var creds).
	KMS *kms.CachedClient
}

// TakePaymentOut is the settled result. The first three fields are the response
// body TopupWithToken has always sent, with the same names, so the browser
// contract is unchanged and an idempotency replay of a record written before
// this split still decodes.
type TakePaymentOut struct {
	// TransactionID is the ledger deposit's id — the receipt for the credit.
	TransactionID string `json:"transactionId"`
	// BalanceCents is the subject's balance read back AFTER the credit, from the
	// same key just credited, so what is returned is what me/balance will report.
	BalanceCents int64 `json:"balanceCents"`
	// Status is "ok" on a settled charge. A failure is a fault, never a status.
	Status string `json:"status"`
	// ProcessorRef is the PROCESSOR's own reference for the charge (Square's
	// payment id). It is the only field here that proves money actually moved at
	// the gateway rather than merely in our ledger, which is why it is returned
	// rather than only logged.
	ProcessorRef string `json:"processorRef,omitempty"`
	// Test reports which bucket this credited: true is a sandbox charge crediting
	// the TEST balance, false is live money. It is stated rather than inferred so
	// no reader has to guess whether a receipt is real.
	Test bool `json:"test"`
}

// PaymentFault is a money-path failure carrying the status the door must answer
// with. The core cannot write an HTTP response — it has no request — but the
// status is part of the money contract (402 declined is not 500 broken, and a
// dunning workflow reads the difference), so it travels with the failure instead
// of being re-derived per door.
type PaymentFault struct {
	// Status is the HTTP status this failure means.
	Status int
	// Message is the caller-safe explanation.
	Message string
	// Err is the underlying cause, for logs. Never sent to the caller verbatim.
	Err error
}

func (f *PaymentFault) Error() string {
	if f.Err != nil {
		return fmt.Sprintf("%s: %v", f.Message, f.Err)
	}
	return f.Message
}

func fault(status int, msg string, err error) *PaymentFault {
	return &PaymentFault{Status: status, Message: msg, Err: err}
}

// TakePayment charges a single-use card token and credits the subject's balance,
// exactly once.
//
// It is the ONE card money move in this package: the browser's top-up and the
// typed op an agent calls are both projections of this function, so the bounds,
// the idempotency derivation and the ledger write cannot drift between them.
//
// The order is deliberate and is the difference between a refund and a
// reconciliation: bounds are checked BEFORE the guard, the guard is taken BEFORE
// the charge, and the credit happens AFTER the charge settles. A failure before
// the charge RELEASES the guard so a later attempt with a fresh nonce is not
// wedged; a failure after it does NOT, because the money already moved and a
// same-key retry must not charge a second time.
//
// The returned balance is read back from the same key just credited, so the
// number handed to the caller is the number the balance endpoint will report
// rather than an optimistic local sum.
func TakePayment(ctx context.Context, org *organization.Organization, in TakePaymentIn) (*TakePaymentOut, *PaymentFault) {
	if org == nil {
		return nil, fault(401, "missing identity headers", nil)
	}
	if in.SourceID == "" {
		return nil, fault(400, "sourceId is required", nil)
	}
	if in.Subject == "" {
		return nil, fault(401, "missing identity headers", nil)
	}

	// Best-effort credential hydration, exactly as the browser path did: a
	// missing KMS client is dev/test with env-var creds, not a failure.
	if in.KMS != nil {
		if err := kms.Hydrate(in.KMS, org); err != nil {
			log.Error("KMS hydration failed for org %q: %v", org.Name, err)
		}
	}

	db := datastore.New(org.Namespaced(ctx))

	// Server-authoritative amount bounds (money-critical). This is the ONLY
	// enforcement: a scripted or agent-driven request never passes through the
	// console's client-side cap, so the floor and ceiling must bind here.
	minCents, maxCents := topupBounds()
	if in.AmountCents < minCents || in.AmountCents > maxCents {
		return nil, fault(400, fmt.Sprintf("amountCents must be between %d and %d", minCents, maxCents), nil)
	}

	cur := normalizeCurrency(in.Currency)

	// ONE idempotency derivation, shared with every other card money move: the
	// caller's explicit key, else the stable request facts — the AMOUNT, never
	// the single-use nonce — bucketed into a coarse window.
	idemKey := in.IdempotencyKey
	if idemKey == "" {
		idemKey = windowKey("amount:" + fmt.Sprint(in.AmountCents) + ":cur:" + string(cur))
	}
	squareKey := gatewayKey("topup", in.Subject, idemKey)

	rec, replay, gerr := idemBegin(db, "billing-topup:"+in.Subject, idemKey)
	if gerr != nil {
		// The guard store is unavailable, so a first attempt cannot be told from a
		// retry. Refuse. Proceeding is not defensible here: a re-tokenized retry
		// carries a FRESH nonce, so in exactly this case the nonce backstop does
		// not exist. Refusing costs a retry; proceeding costs a second real charge.
		log.Error("topup idempotency Begin failed (org=%s): %v", in.Subject, gerr)
		return nil, fault(503, "billing is temporarily unavailable; please retry", gerr)
	}
	if replay {
		if rec.Status == idempotencykey.StatusCompleted && rec.Response != "" {
			out, derr := decodeTakePaymentOut(rec.Response)
			if derr != nil {
				return nil, fault(500, "could not read the completed payment record", derr)
			}
			return out, nil
		}
		return nil, fault(409, "top-up already in progress", nil)
	}
	// abandon releases the guard so a later attempt (with a fresh nonce) is not
	// wedged. Called ONLY on pre-credit failures where no balance moved; the dead
	// nonce prevents any double-charge on a same-nonce retry.
	abandon := func() {
		if rec != nil {
			_ = rec.Delete()
		}
	}

	chargeReq := processor.PaymentRequest{
		Token:          in.SourceID,
		Amount:         currency.Cents(in.AmountCents),
		Currency:       cur,
		IdempotencyKey: squareKey,
		Description:    fmt.Sprintf("Top-up %d %s for org %s", in.AmountCents, cur, in.Subject),
	}

	// The per-org processor registry, built from the org's KMS-hydrated payment
	// credentials. The global registry holds unconfigured singletons, so a charge
	// must go through the org-scoped one to reach the provider with real creds.
	reg := processorsForOrg(org)
	proc, err := reg.SelectProcessor(ctx, chargeReq)
	if err != nil {
		abandon()
		log.Error("No processor available for token topup: %v", err)
		return nil, fault(422, "no payment processor available", err)
	}

	result, err := proc.Charge(ctx, chargeReq)
	if err != nil {
		abandon()
		log.Error("Charge failed for token topup (org=%s): %v", in.Subject, err)
		return nil, fault(402, "charge failed", err)
	}
	if !result.Success {
		abandon()
		msg := result.ErrorMessage
		if msg == "" {
			msg = "charge declined"
		}
		return nil, fault(402, msg, nil)
	}

	// Credit the canonical balance for the subject the caller's door bounded.
	trans := transaction.New(db)
	trans.Type = transaction.Deposit
	trans.DestinationId = in.Subject
	trans.DestinationKind = "iam-user"
	trans.Currency = cur
	trans.Amount = currency.Cents(in.AmountCents)
	trans.Notes = fmt.Sprintf("Top-up via %s (ref: %s)", proc.Type(), result.ProcessorRef)
	trans.Tags = "topup"

	// Ledger test-ness MUST follow the charge environment (org.TestMode): a Square
	// sandbox charge credits the TEST bucket, never the live spendable one.
	// test == credit-bucket == read-bucket == charge-env.
	test := org.TestMode()
	trans.Test = test

	// The card token was charged (result.Success): a settled payment IS the mint
	// authority, so authorize THIS write at the ledger sink (money-in == credit).
	trans.SetContext(mintauth.WithAuthorized(trans.Context()))
	if err := trans.Create(); err != nil {
		// Charge settled but the credit failed. Leave the guard STARTED so a
		// same-key retry 409s rather than re-charging, and log for manual
		// reconciliation. The money moved; the dead nonce blocks a re-charge.
		log.Error("RECONCILE: charge succeeded (ref=%s) but deposit failed for org %s: %v",
			result.ProcessorRef, in.Subject, err)
		return nil, fault(500, "charge succeeded but balance credit failed; contact support", err)
	}

	// Read back the SAME key just credited so the returned balance matches what
	// me/balance reports (read == credit).
	var balanceCents currency.Cents
	if datas, derr := util.GetTransactionsByCurrency(org.Namespaced(ctx), in.Subject, "iam-user", cur, test); derr == nil {
		if data, ok := datas.Data[cur]; ok {
			balanceCents = data.Balance
		}
	}

	out := &TakePaymentOut{
		TransactionID: trans.Id(),
		BalanceCents:  int64(balanceCents),
		Status:        "ok",
		ProcessorRef:  result.ProcessorRef,
		Test:          test,
	}
	// Seal the guard with the exact success body so a retry replays it verbatim —
	// no second charge, identical answer.
	if rec != nil {
		if body, mErr := encodeTakePaymentOut(out); mErr == nil {
			_ = idempotencykey.Complete(rec, body)
		}
	}
	return out, nil
}

// PaymentRecord is a settled payment as the ledger holds it — the receipt a
// caller reads back after TakePayment returned its id.
type PaymentRecord struct {
	// ID is the ledger transaction id, which is what TakePayment returned.
	ID string `json:"id"`
	// Subject is the billing key this payment credited.
	Subject string `json:"subject"`
	// AmountCents is the credited amount in whole cents.
	AmountCents int64 `json:"amountCents"`
	// Currency is the ISO code.
	Currency string `json:"currency"`
	// Status is the payment's state. A deposit in this ledger is written only
	// AFTER the processor settled the charge, so a record that exists is a
	// payment that succeeded — "settled" is the only truthful value here, and it
	// is stated rather than left for a reader to assume.
	Status string `json:"status"`
	// Test reports whether this credited the TEST bucket (a sandbox charge) or
	// live money. A receipt that does not say which is a receipt that can be
	// mistaken for the other.
	Test bool `json:"test"`
	// Notes is the ledger memo, which carries the processor and its reference.
	Notes string `json:"notes,omitempty"`
	// CreatedAt is when the credit was written, RFC3339.
	CreatedAt string `json:"createdAt,omitempty"`
}

// ReadPayment loads one settled payment out of the org's own ledger namespace.
//
// The org scopes the read by construction: the datastore is namespaced to it, so
// an id from another tenant simply is not found rather than being found and then
// filtered — there is no window in which the wrong row is in hand. A row that is
// not a deposit is not a payment and is refused as not-found, so this cannot be
// used to enumerate usage debits.
func ReadPayment(ctx context.Context, org *organization.Organization, id string) (*PaymentRecord, *PaymentFault) {
	if org == nil {
		return nil, fault(401, "missing identity headers", nil)
	}
	if strings.TrimSpace(id) == "" {
		return nil, fault(400, "payment id is required", nil)
	}
	db := datastore.New(org.Namespaced(ctx))
	t := transaction.New(db)
	if err := t.GetById(strings.TrimSpace(id)); err != nil {
		return nil, fault(404, "payment not found", err)
	}
	if t.Type != transaction.Deposit {
		return nil, fault(404, "payment not found", nil)
	}
	rec := &PaymentRecord{
		ID:          t.Id(),
		Subject:     t.DestinationId,
		AmountCents: int64(t.Amount),
		Currency:    string(t.Currency),
		Status:      "settled",
		Test:        t.Test,
		Notes:       t.Notes,
	}
	if !t.CreatedAt.IsZero() {
		rec.CreatedAt = t.CreatedAt.UTC().Format(time.RFC3339)
	}
	return rec, nil
}

// normalizeCurrency is the ONE currency default, shared by both doors so a
// missing currency cannot mean usd on one and empty on the other.
func normalizeCurrency(code string) currency.Type {
	cur := currency.Type(strings.ToLower(strings.TrimSpace(code)))
	if cur == "" {
		return currency.USD
	}
	return cur
}

// encodeTakePaymentOut renders the sealed idempotency body. It is the SAME
// marshal the fresh answer uses, which is what makes a replay byte-identical to
// the original rather than merely equivalent.
func encodeTakePaymentOut(out *TakePaymentOut) (string, error) {
	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// decodeTakePaymentOut reads a sealed body back. A record written before the
// core split carries only the original three fields; the rest decode as their
// zero values, which is honest — that receipt genuinely does not record them.
func decodeTakePaymentOut(body string) (*TakePaymentOut, error) {
	var out TakePaymentOut
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
