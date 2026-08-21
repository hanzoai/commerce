package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/thirdparty/kms"
	"github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/json/http"
)

// preAuthVerifier is implemented by processors that support pre-authorization
// cancellation (e.g. Square). Allows voiding a hold immediately after verifying
// the card is real.
type preAuthVerifier interface {
	processor.PaymentProcessor
	CancelAuthorization(ctx context.Context, paymentID string) error
}

// verifyCardWithPreAuth does a $1.00 Square pre-auth against the provided nonce
// to confirm the card is real and chargeable, then immediately voids it.
// Returns an error (with a user-facing message) if the card is declined.
//
// reg must be the per-org registry (payment.ProcessorsForOrg) so the Square
// processor carries the org's real credentials; the global registry holds an
// empty Square singleton and would silently skip verification.
func verifyCardWithPreAuth(ctx context.Context, reg *processor.Registry, nonce, customerID string) error {
	p, err := reg.Get(processor.Square)
	if err != nil {
		// Square not configured — skip pre-auth (not an error, just log)
		return nil
	}

	verifier, ok := p.(preAuthVerifier)
	if !ok || !verifier.IsAvailable(ctx) {
		return nil
	}

	// $1.00 pre-auth to verify the card is real and has available credit.
	// Don't pass CustomerID — Square requires a Square-generated customer ID,
	// not our IAM user ID. The nonce alone is sufficient for verification.
	result, err := verifier.Authorize(ctx, processor.PaymentRequest{
		Amount:      currency.Cents(100), // $1.00
		Currency:    currency.USD,
		Token:       nonce,
		Description: "Card verification hold (will be voided immediately)",
	})
	if err != nil || !result.Success {
		// Return a clean, user-friendly error instead of raw API responses.
		reason := parseCardDeclineReason(result, err)
		return fmt.Errorf("%s", reason)
	}

	// Immediately void the authorization — we only needed to verify the card.
	if voidErr := verifier.CancelAuthorization(ctx, result.TransactionID); voidErr != nil {
		// Non-fatal: the hold will expire on its own (Square: ~7 days).
		// Log but don't block the user.
		log.Warn("Failed to void pre-auth %s: %v", result.TransactionID, voidErr)
	}

	return nil
}

// parseCardDeclineReason returns a single clean sentence explaining why the card was declined.
func parseCardDeclineReason(result *processor.PaymentResult, err error) string {
	if result == nil && err != nil {
		if strings.Contains(err.Error(), "timeout") {
			return "Card verification timed out. Please try again."
		}
		return "Unable to verify card. Please try again or use a different card."
	}

	msg := ""
	if result != nil {
		msg = result.ErrorMessage
	}
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(lower, "insufficient_funds"):
		return "Card declined — insufficient funds."
	case strings.Contains(lower, "transaction_limit"):
		return "Card declined — transaction limit reached. Please try a different card."
	case strings.Contains(lower, "address_verification_failure") || strings.Contains(lower, "avs_rejected"):
		return "Card declined — billing address does not match. Please check your address and try again."
	case strings.Contains(lower, "cvv") || strings.Contains(lower, "cvc"):
		return "Card declined — incorrect security code (CVV)."
	case strings.Contains(lower, "expired"):
		return "Card declined — card is expired."
	case strings.Contains(lower, "invalid_card") || strings.Contains(lower, "invalid_account"):
		return "Card declined — invalid card number."
	case strings.Contains(lower, "stolen") || strings.Contains(lower, "lost"):
		return "Card declined — please contact your bank."
	case strings.Contains(lower, "do_not_honor") || strings.Contains(lower, "generic_decline"):
		return "Card declined by your bank. Please try a different card or contact your bank."
	case msg != "":
		return "Card declined. Please try a different card."
	default:
		return "Unable to verify card. Please try again or use a different card."
	}
}

// The refusals these cores make, as VALUES. A core has no request to answer on,
// so it says which KIND of refusal happened and the door it was reached through
// picks the status — the same split subValidationError makes for subscriptions.

// errNoMethod is the single answer to "not there, or not yours". A row the
// caller's namespace does not hold and a row that belongs to another subject
// refuse identically, so the refusal can never be read as proof that a card id
// is real.
var errNoMethod = errors.New("payment method not found")

// IsMethodNotFound reports whether err is that one answer. A caller outside this
// package has to answer it exactly as the door does — not found, no detail — and
// an unexported sentinel cannot be asked about.
func IsMethodNotFound(err error) bool { return errors.Is(err, errNoMethod) }

// methodValidationError marks a refusal of what the CALLER asked for: a request
// naming no customer, a card with nothing to charge, a body that is not the shape
// it claims.
type methodValidationError struct{ msg string }

func (e methodValidationError) Error() string { return e.msg }

// IsMethodRefused reports whether err is that refusal, as distinct from the store
// failing. The asker can fix the first and can do nothing about the second, which
// is why they carry different statuses — and why a caller that could not tell
// them apart would report every typo as an outage.
func IsMethodRefused(err error) bool {
	var ve methodValidationError
	return errors.As(err, &ve)
}

// declineError marks a card the processor refused. Its msg is already the
// customer-facing sentence parseCardDeclineReason produced, which is why the door
// sends it verbatim rather than writing a second one.
type declineError struct{ msg string }

func (e declineError) Error() string { return e.msg }

// IsCardDeclined reports whether err is the bank's answer rather than ours —
// neither of the other two is true of it: the request was well formed and nothing
// here failed. The only useful thing to do with it is offer another card, so it
// must be distinguishable from both.
func IsCardDeclined(err error) bool {
	var de declineError
	return errors.As(err, &de)
}

// CreateMethodIn is the whole input of saving a payment method: the body a
// browser posts, and the same fields a peer fills in over the internal plane.
type CreateMethodIn struct {
	CustomerId     string                            `json:"customerId"`
	Type           string                            `json:"type"` // "card" | "bank_account" | "crypto" | "wire" | "paypal" | "balance"
	Card           *paymentmethod.CardDetails        `json:"card,omitempty"`
	BankAccount    *paymentmethod.BankAccountDetails `json:"bankAccount,omitempty"`
	Crypto         *paymentmethod.CryptoDetails      `json:"crypto,omitempty"`
	Wire           *paymentmethod.WireDetails        `json:"wire,omitempty"`
	PayPal         *paymentmethod.PayPalDetails      `json:"paypal,omitempty"`
	BillingAddress *types.Address                    `json:"billingAddress,omitempty"`
	ProviderRef    string                            `json:"providerRef,omitempty"`
	ProviderType   string                            `json:"providerType,omitempty"`
	// Metadata is the caller's free-form extras, carried raw so the input crosses
	// the typed plane the same way the answer does.
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// CreateMethod saves a payment method for a subject and returns it, plus whether
// the row is NEW: saving a card that is already on file answers with the row that
// already holds it rather than stacking a duplicate, and the caller has to be
// able to tell which happened.
//
// It takes values rather than a request because the browser's door is not the
// only place this is asked from — a peer holding no datastore asks it over the
// internal plane — and a second implementation of "vault a nonce and persist a
// method" is a second place for the card-on-file dedupe to go missing.
//
// email is the caller's own address, resolved at the door from its credential: it
// names the Square customer profile, and a core must never read an identity it
// was not handed. creds hydrates the org's processor credentials; nil is
// legitimate (dev/tests, env-var creds).
func CreateMethod(ctx context.Context, org *organization.Organization, email string, creds *kms.CachedClient, in CreateMethodIn) (*Method, bool, error) {
	if org == nil {
		return nil, false, errors.New("payment methods: no organization")
	}

	// Hydrate payment credentials from KMS so the per-org Square processor
	// used for card verification carries real credentials.
	if creds != nil {
		if err := kms.Hydrate(creds, org); err != nil {
			log.Error("KMS hydration failed for org %q: %v", org.Name, err)
		}
	}

	db := datastore.New(org.Namespaced(ctx))

	var meta map[string]interface{}
	if len(in.Metadata) > 0 {
		if err := json.Unmarshal(in.Metadata, &meta); err != nil {
			return nil, false, methodValidationError{"invalid request body"}
		}
	}

	if in.CustomerId == "" {
		return nil, false, methodValidationError{"customerId is required"}
	}

	// When a Square nonce/sourceId is provided, vault it as a reusable
	// card-on-file so the saved method can be charged later (top-ups, plan
	// checkout, auto-recharge). saveCard is the ONE constructor: it validates
	// by vaulting (the nonce is single-use — a separate pre-auth would consume
	// it), stamps the card's brand/last4/expiry on the row, and returns the
	// EXISTING row instead of stacking a duplicate when the same card is saved
	// again. Use the per-org registry so Square carries the org's credentials.
	if in.ProviderRef != "" {
		reg := payment.ProcessorsForOrg(org)
		if cp, ok := squareCustomerProcessorFrom(reg); ok {
			pm, created, err := saveCard(ctx, db, cp, in.CustomerId, strings.TrimSpace(email), in.ProviderRef)
			if err != nil {
				return nil, false, declineError{parseCardDeclineReason(&processor.PaymentResult{ErrorMessage: err.Error()}, err)}
			}
			// Caller-supplied extras land on whichever row answered.
			if in.BillingAddress != nil {
				pm.BillingAddress = in.BillingAddress
			}
			for k, v := range meta {
				if pm.Metadata == nil {
					pm.Metadata = map[string]interface{}{}
				}
				pm.Metadata[k] = v
			}
			if err := pm.Update(); err != nil {
				log.Error("Failed to update payment method extras: %v", err)
			}
			m := methodOf(pm)
			return &m, created, nil
		} else if err := verifyCardWithPreAuth(ctx, reg, in.ProviderRef, in.CustomerId); err != nil {
			// Square not available as a customer processor — fall back to the
			// legacy verify-only flow (the saved card is NOT reusable).
			return nil, false, declineError{err.Error()}
		}
	}

	// A CARD with nothing to charge is not a payment method — it is a row that
	// looks like one. Without a providerRef there is no nonce to vault and no
	// card-on-file id to store, so what lands is a `type:"card"` record with no
	// brand, no last4 and no way to bill it: it shows up in the customer's saved
	// cards, and any renewal that picks it fails on a method that was never real.
	// Measured on the live portal route the moment it opened — a POST carrying
	// only {"type":"card"} answered 201 and created exactly that.
	//
	// Refuse it here rather than at the edge, because every caller reaches this
	// one constructor. Other method types (bank/wire/paypal/crypto) legitimately
	// carry their details in the body instead, so the rule is scoped to cards.
	if strings.EqualFold(strings.TrimSpace(in.Type), "card") &&
		strings.TrimSpace(in.ProviderRef) == "" && in.Card == nil {
		return nil, false, methodValidationError{"a card payment method requires a tokenized card (providerRef)"}
	}

	pm := paymentmethod.New(db)
	pm.CustomerId = in.CustomerId
	pm.UserId = in.CustomerId
	if in.Type != "" {
		pm.Type = in.Type
	}
	pm.Card = in.Card
	pm.BankAccount = in.BankAccount
	pm.Crypto = in.Crypto
	pm.Wire = in.Wire
	pm.PayPal = in.PayPal
	pm.BillingAddress = in.BillingAddress
	pm.ProviderRef = in.ProviderRef
	pm.ProviderType = in.ProviderType

	switch {
	case in.Card != nil:
		pm.Name = in.Card.Brand + " ending in " + in.Card.Last4
	case in.Crypto != nil:
		if in.Crypto.Label != "" {
			pm.Name = in.Crypto.Label
		} else if in.Crypto.Network != "" {
			pm.Name = in.Crypto.Network + " wallet"
		} else {
			pm.Name = "Crypto wallet"
		}
	case in.Wire != nil:
		if in.Wire.BankName != "" {
			pm.Name = in.Wire.BankName + " wire"
		} else {
			pm.Name = "Bank wire"
		}
	case in.PayPal != nil:
		pm.Name = strings.TrimSpace("PayPal " + in.PayPal.Email)
	}

	if meta == nil {
		meta = make(map[string]interface{})
	}
	if in.ProviderRef != "" {
		meta["squareVerified"] = true
	}
	pm.Metadata = meta

	if err := pm.Create(); err != nil {
		return nil, false, err
	}

	m := methodOf(pm)
	return &m, true, nil
}

// CreatePaymentMethod creates and attaches a payment method to a customer.
//
//	POST /v1/billing/methods
func CreatePaymentMethod(c *zip.Ctx) error {
	// The OK form: IAMTokenRequired falls through WITHOUT setting the
	// "organization" local when the gateway named no principal, and the MustGet
	// form panics there — a 500 with no body, after the money has moved. Refuse
	// before touching anything. See SubscribeWithCard, where this cost a $99
	// charge with no subscription behind it.
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return http.Fail(c, 401, "sign in to add a payment method", nil)
	}

	var in CreateMethodIn
	if err := c.Bind(&in); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	// Both come off the request and neither may be re-derived in the core: the
	// email is who the credential says the caller is, the KMS client is this
	// request's cached one.
	email, _ := c.Locals("iam_email").(string)
	creds, _ := c.Locals("kms").(*kms.CachedClient)

	method, created, err := CreateMethod(c.Context(), org, email, creds, in)
	if err != nil {
		switch {
		case IsMethodRefused(err):
			return http.Fail(c, 400, err.Error(), nil)
		case IsCardDeclined(err):
			return http.Fail(c, 402, err.Error(), nil)
		}
		log.Error("Failed to create payment method: %v", err, c)
		return http.Fail(c, 500, "failed to create payment method", err)
	}

	if !created {
		// The same card was already on file: the answer is that row, and 200
		// states "nothing new was created" honestly.
		return c.JSON(200, method)
	}
	return c.JSON(201, method)
}

// GetPaymentMethod retrieves a payment method by ID.
//
//	GET /v1/billing/methods/:id
func GetPaymentMethod(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	pm := paymentmethod.New(db)
	if err := pm.GetById(c.Param("id")); err != nil {
		return http.Fail(c, 404, "payment method not found", err)
	}

	// Intra-org IDOR guard (#43a, per-user). This user-group route admits ANY
	// authenticated org member (no Admin mask), and the :id path-param — which
	// EdgeAuth does not pin — can name a DIFFERENT subject's saved card
	// (last4/brand/billing address/providerRef/squareCardId) inside the caller's
	// namespace. Non-privileged callers may read only their own subject's method;
	// 404 (not 403) so card ids can't be probed.
	if !callerMayReachBillingSubject(c, pm.CustomerId, pm.UserId) {
		return http.Fail(c, 404, "payment method not found", nil)
	}

	return c.JSON(200, methodOf(pm))
}

// ListMethods is a subject's saved payment methods, healed and rendered — the
// QUERY, with no HTTP in it.
//
// It takes values rather than a request because two doors already ask it (a
// customer's own list, and the portal list a proxying host reads) and a peer
// holding no datastore asks it over the internal plane. Three copies of one query
// is how a saved-cards panel and a renewal come to disagree about which cards
// exist.
//
// An empty subject means "every method in the org" — what an absent customerId
// has always meant on the privileged path. WHO may ask that is settled at the
// door, from the credential, and never re-derived here. An empty kind means "any
// type", which is what an absent ?type has always meant.
//
// creds hydrates the org's processor credentials before card facts are healed
// onto rows vaulted before the vault reported them; nil is legitimate (dev/tests,
// env-var creds). Nothing is hydrated when the subject holds no methods, because
// this read sits in front of a card field and a slow KMS must not stall it.
//
// The list is always a list, never nil: an org with no saved cards answers [].
func ListMethods(ctx context.Context, org *organization.Organization, subject, kind string, creds *kms.CachedClient) ([]Method, error) {
	methods := make([]Method, 0)
	if org == nil {
		return methods, errors.New("payment methods: no organization")
	}
	db := datastore.New(org.Namespaced(ctx))

	q := paymentmethod.Query(db).Ancestor(db.NewKey("synckey", "", 1, nil))
	if subject != "" {
		q = q.Filter("CustomerId=", subject)
	}
	if kind != "" {
		q = q.Filter("Type=", kind)
	}

	rows := make([]*paymentmethod.PaymentMethod, 0)
	if _, err := q.Order("-Created").GetAll(&rows); err != nil {
		return methods, err
	}

	// Backfill brand/last4/expiry onto vaulted rows saved before the vault
	// reported card facts, so a customer can tell their saved cards apart.
	// Best-effort — see heal.
	if len(rows) > 0 {
		if creds != nil {
			if err := kms.Hydrate(creds, org); err != nil {
				log.Error("KMS hydration failed for org %q: %v", org.Name, err)
			}
		}
		heal(ctx, payment.ProcessorsForOrg(org), rows)
	}

	for _, pm := range rows {
		methods = append(methods, methodOf(pm))
	}
	return methods, nil
}

// ListPaymentMethods lists payment methods for a customer.
//
//	GET /v1/billing/methods?customerId=...&type=...
func ListPaymentMethods(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)

	// Intra-org IDOR guard (#43a, enumeration). A non-privileged org member may
	// only ever list its OWN subject's methods. EdgeAuth pins a PRESENT
	// customerId/user query param to the org slug, but an ABSENT filter would
	// return EVERY method in the namespace (incl. any service-token-created
	// per-user records) — so force the subject filter for non-privileged callers,
	// failing closed when no subject resolves. Privileged callers (service token /
	// admin / global admin) keep the explicit client-supplied filter.
	var subject string
	if isPrivilegedBillingCaller(c) {
		if customerId := c.Query("customerId"); customerId != "" {
			subject = customerId
		} else if user := c.Query("user"); user != "" {
			subject = user
		}
	} else if subject = orgBillingKey(c); subject == "" {
		return c.JSON(200, []Method{})
	}

	creds, _ := c.Locals("kms").(*kms.CachedClient)
	methods, err := ListMethods(c.Context(), org, subject, c.Query("type"), creds)
	if err != nil {
		// A store that cannot be read has always answered an honest empty list
		// here rather than a 500: the saved-cards panel renders empty instead of
		// breaking the page around it.
		log.Error("Failed to list payment methods: %v", err, c)
	}
	return c.JSON(200, methods)
}

type updatePaymentMethodRequest struct {
	BillingAddress *types.Address         `json:"billingAddress,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// UpdatePaymentMethod updates a payment method.
//
//	PATCH /v1/billing/methods/:id
func UpdatePaymentMethod(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	pm := paymentmethod.New(db)
	if err := pm.GetById(c.Param("id")); err != nil {
		return http.Fail(c, 404, "payment method not found", err)
	}

	// Intra-org IDOR guard (#43a, per-user). Mutating another subject's card
	// (billing address / metadata — including the squareCustomerId/squareCardId
	// that route later charges) is a cross-subject tamper the unpinned :id can
	// reach inside the caller's namespace. Non-privileged callers may update only
	// their own subject's method; 404 so ids can't be probed.
	if !callerMayReachBillingSubject(c, pm.CustomerId, pm.UserId) {
		return http.Fail(c, 404, "payment method not found", nil)
	}

	var req updatePaymentMethodRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	if req.BillingAddress != nil {
		pm.BillingAddress = req.BillingAddress
	}
	if req.Metadata != nil {
		pm.Metadata = req.Metadata
	}

	if err := pm.Update(); err != nil {
		log.Error("Failed to update payment method: %v", err, c)
		return http.Fail(c, 500, "failed to update payment method", err)
	}

	return c.JSON(200, methodOf(pm))
}

// Detachment is the answer to removing a saved method: the id that is now gone.
type Detachment struct {
	Deleted bool   `json:"deleted"`
	Id      string `json:"id"`
}

// DetachMethod removes a subject's saved method: the Square card-on-file first,
// best-effort so the vault stays in step, then the local row.
//
// It takes values rather than a request so the doors that publish this removal
// and a peer on the internal plane share one implementation of it. What it must
// NOT take is the caller: subject is the billing key the door already proved the
// caller owns, and privileged is the service or admin caller that may act on any
// subject inside the org. Authority decided twice is authority that eventually
// disagrees with itself.
//
// A method the org's namespace does not hold and a method belonging to another
// subject both answer errNoMethod, so the refusal cannot be used to prove a card
// id exists.
//
// creds hydrates the org's processor credentials, and only when the row actually
// carries a Square card to detach; nil is legitimate (dev/tests, env-var creds).
func DetachMethod(ctx context.Context, org *organization.Organization, id, subject string, privileged bool, creds *kms.CachedClient) (*Detachment, error) {
	if org == nil {
		return nil, errors.New("payment methods: no organization")
	}
	db := datastore.New(org.Namespaced(ctx))

	pm := paymentmethod.New(db)
	if err := pm.GetById(id); err != nil {
		return nil, fmt.Errorf("%w: %v", errNoMethod, err)
	}

	// Intra-org IDOR guard (#43a, per-user). Detaching another subject's card
	// (soft-delete + Square card-on-file removal) breaks their saved-card charges
	// and auto-recharge — a cross-subject mutation the unpinned :id can reach in
	// the caller's namespace. A caller with no subject at all reaches nothing.
	if !privileged && (subject == "" ||
		(!strings.EqualFold(strings.TrimSpace(pm.CustomerId), subject) &&
			!strings.EqualFold(strings.TrimSpace(pm.UserId), subject))) {
		return nil, errNoMethod
	}

	// Best-effort: detach the card-on-file from Square so its vault stays in
	// sync. Non-fatal — the local record is still soft-deleted on failure.
	if pm.Metadata != nil {
		custID, _ := pm.Metadata["squareCustomerId"].(string)
		cardID, _ := pm.Metadata["squareCardId"].(string)
		if custID != "" && cardID != "" {
			if creds != nil {
				if err := kms.Hydrate(creds, org); err != nil {
					log.Error("KMS hydration failed for org %q: %v", org.Name, err)
				}
			}
			reg := payment.ProcessorsForOrg(org)
			if cp, ok := squareCustomerProcessorFrom(reg); ok {
				if err := cp.RemovePaymentMethod(ctx, custID, cardID); err != nil {
					log.Warn("Failed to remove Square card %s for customer %s: %v", cardID, custID, err)
				}
			}
		}
	}

	if err := pm.Delete(); err != nil {
		return nil, err
	}

	return &Detachment{Deleted: true, Id: pm.Id()}, nil
}

// DetachPaymentMethod detaches (soft-deletes) a payment method.
//
//	DELETE /v1/billing/methods/:id
func DetachPaymentMethod(c *zip.Ctx) error {
	// The OK form: IAMTokenRequired falls through WITHOUT setting the
	// "organization" local when the gateway named no principal, and the MustGet
	// form panics there — a 500 with no body, after the money has moved. Refuse
	// before touching anything. See SubscribeWithCard, where this cost a $99
	// charge with no subscription behind it.
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return http.Fail(c, 401, "sign in to remove a payment method", nil)
	}

	creds, _ := c.Locals("kms").(*kms.CachedClient)
	detached, err := DetachMethod(c.Context(), org, c.Param("id"),
		orgBillingKey(c), isPrivilegedBillingCaller(c), creds)
	if err != nil {
		if IsMethodNotFound(err) {
			return http.Fail(c, 404, "payment method not found", err)
		}
		log.Error("Failed to detach payment method: %v", err, c)
		return http.Fail(c, 500, "failed to detach payment method", err)
	}

	return c.JSON(200, detached)
}

type setDefaultRequest struct {
	PaymentMethodId string `json:"paymentMethodId"`
}

// SetDefaultPaymentMethod sets the default payment method for a customer.
//
//	POST /v1/billing/customers/:id/default-payment-method
func SetDefaultPaymentMethod(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	customerId := c.Param("id")

	// Intra-org IDOR guard (#43a, per-user). The :id customer path-param scopes the
	// default-unset sweep below; a non-privileged caller must not clear another
	// subject's default flags. 404 keeps customer ids unprobeable.
	if !callerMayReachBillingSubject(c, customerId) {
		return http.Fail(c, 404, "payment method not found", nil)
	}

	var req setDefaultRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	// Unset any existing default for this customer
	rootKey := db.NewKey("synckey", "", 1, nil)
	iter := paymentmethod.Query(db).Ancestor(rootKey).
		Filter("CustomerId=", customerId).
		Filter("IsDefault=", true).
		Run()

	for {
		existing := paymentmethod.New(db)
		if _, err := iter.Next(existing); err != nil {
			break
		}
		existing.IsDefault = false
		_ = existing.Update()
	}

	// Set the new default
	pm := paymentmethod.New(db)
	if err := pm.GetById(req.PaymentMethodId); err != nil {
		return http.Fail(c, 404, "payment method not found", err)
	}

	// Intra-org IDOR guard (#43a, per-user). paymentMethodId is an unpinned body
	// field that can name a DIFFERENT subject's card; guard it too so a caller
	// can't flip another subject's card to default. 404, no existence oracle.
	if !callerMayReachBillingSubject(c, pm.CustomerId, pm.UserId) {
		return http.Fail(c, 404, "payment method not found", nil)
	}

	pm.IsDefault = true
	if err := pm.Update(); err != nil {
		log.Error("Failed to set default payment method: %v", err, c)
		return http.Fail(c, 500, "failed to set default", err)
	}

	return c.JSON(200, methodOf(pm))
}

// Method is a saved payment instrument as every billing surface states it: the
// row's identity, what kind of instrument it is, and only those details the
// instrument actually carries.
//
// The json tags ARE the wire contract, key for key with the hand-built map they
// replace — a detail a row does not hold stays absent rather than arriving null,
// which is what lets a client test presence instead of emptiness. Being a type
// rather than a map is what lets the same answer cross the internal plane.
type Method struct {
	Id         string    `json:"id"`
	CustomerId string    `json:"customerId"`
	Type       string    `json:"type"`
	IsDefault  bool      `json:"isDefault"`
	Created    time.Time `json:"created"`

	Name           string                            `json:"name,omitempty"`
	Card           *paymentmethod.CardDetails        `json:"card,omitempty"`
	BankAccount    *paymentmethod.BankAccountDetails `json:"bankAccount,omitempty"`
	Crypto         *paymentmethod.CryptoDetails      `json:"crypto,omitempty"`
	Wire           *paymentmethod.WireDetails        `json:"wire,omitempty"`
	PayPal         *paymentmethod.PayPalDetails      `json:"paypal,omitempty"`
	BillingAddress *types.Address                    `json:"billingAddress,omitempty"`
	ProviderRef    string                            `json:"providerRef,omitempty"`
	// Metadata is the row's free-form extras, carried raw: it renders exactly as
	// the map did, including an empty object for a row that holds an empty one,
	// and raw JSON is the one free-form shape the typed plane can carry.
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// methodOf states a stored row as the Method every door answers with.
func methodOf(pm *paymentmethod.PaymentMethod) Method {
	m := Method{
		Id:             pm.Id(),
		CustomerId:     pm.CustomerId,
		Type:           pm.Type,
		IsDefault:      pm.IsDefault,
		Created:        pm.Created,
		Name:           pm.Name,
		Card:           pm.Card,
		BankAccount:    pm.BankAccount,
		Crypto:         pm.Crypto,
		Wire:           pm.Wire,
		PayPal:         pm.PayPal,
		BillingAddress: pm.BillingAddress,
		ProviderRef:    pm.ProviderRef,
	}
	if pm.Metadata != nil {
		if raw, err := json.Marshal(pm.Metadata); err == nil {
			m.Metadata = raw
		}
	}
	return m
}
