package billing

import (
	"context"
	"fmt"
	"strings"

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

type createPaymentMethodRequest struct {
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
	Metadata       map[string]interface{}            `json:"metadata,omitempty"`
}

// CreatePaymentMethod creates and attaches a payment method to a customer.
//
//	POST /v1/billing/methods
func CreatePaymentMethod(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)

	// Hydrate payment credentials from KMS so the per-org Square processor
	// used for card verification carries real credentials.
	if v := c.Locals("kms"); v != nil {
		if kmsClient, ok := v.(*kms.CachedClient); ok {
			if err := kms.Hydrate(kmsClient, org); err != nil {
				log.Error("KMS hydration failed for org %q: %v", org.Name, err, c)
			}
		}
	}

	db := datastore.New(org.Namespaced(c.Context()))

	var req createPaymentMethodRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	if req.CustomerId == "" {
		return http.Fail(c, 400, "customerId is required", nil)
	}

	// When a Square nonce/sourceId is provided, vault it as a reusable
	// card-on-file so the saved method can be charged later (top-ups, plan
	// checkout, auto-recharge). saveCard is the ONE constructor: it validates
	// by vaulting (the nonce is single-use — a separate pre-auth would consume
	// it), stamps the card's brand/last4/expiry on the row, and returns the
	// EXISTING row instead of stacking a duplicate when the same card is saved
	// again. Use the per-org registry so Square carries the org's credentials.
	if req.ProviderRef != "" {
		reg := payment.ProcessorsForOrg(org)
		if cp, ok := squareCustomerProcessorFrom(reg); ok {
			iamEmail, _ := c.Locals("iam_email").(string)
			pm, created, err := saveCard(c.Context(), db, cp, req.CustomerId, strings.TrimSpace(iamEmail), req.ProviderRef)
			if err != nil {
				return http.Fail(c, 402, parseCardDeclineReason(&processor.PaymentResult{ErrorMessage: err.Error()}, err), nil)
			}
			// Caller-supplied extras land on whichever row answered.
			if req.BillingAddress != nil {
				pm.BillingAddress = req.BillingAddress
			}
			for k, v := range req.Metadata {
				if pm.Metadata == nil {
					pm.Metadata = map[string]interface{}{}
				}
				pm.Metadata[k] = v
			}
			if err := pm.Update(); err != nil {
				log.Error("Failed to update payment method extras: %v", err, c)
			}
			status := 201
			if !created {
				// The same card was already on file: the answer is that row, and
				// 200 states "nothing new was created" honestly.
				status = 200
			}
			return c.JSON(status, paymentMethodResponse(pm))
		} else if err := verifyCardWithPreAuth(c.Context(), reg, req.ProviderRef, req.CustomerId); err != nil {
			// Square not available as a customer processor — fall back to the
			// legacy verify-only flow (the saved card is NOT reusable).
			return http.Fail(c, 402, err.Error(), nil)
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
	if strings.EqualFold(strings.TrimSpace(req.Type), "card") &&
		strings.TrimSpace(req.ProviderRef) == "" && req.Card == nil {
		return http.Fail(c, 400, "a card payment method requires a tokenized card (providerRef)", nil)
	}

	pm := paymentmethod.New(db)
	pm.CustomerId = req.CustomerId
	pm.UserId = req.CustomerId
	if req.Type != "" {
		pm.Type = req.Type
	}
	pm.Card = req.Card
	pm.BankAccount = req.BankAccount
	pm.Crypto = req.Crypto
	pm.Wire = req.Wire
	pm.PayPal = req.PayPal
	pm.BillingAddress = req.BillingAddress
	pm.ProviderRef = req.ProviderRef
	pm.ProviderType = req.ProviderType

	switch {
	case req.Card != nil:
		pm.Name = req.Card.Brand + " ending in " + req.Card.Last4
	case req.Crypto != nil:
		if req.Crypto.Label != "" {
			pm.Name = req.Crypto.Label
		} else if req.Crypto.Network != "" {
			pm.Name = req.Crypto.Network + " wallet"
		} else {
			pm.Name = "Crypto wallet"
		}
	case req.Wire != nil:
		if req.Wire.BankName != "" {
			pm.Name = req.Wire.BankName + " wire"
		} else {
			pm.Name = "Bank wire"
		}
	case req.PayPal != nil:
		pm.Name = strings.TrimSpace("PayPal " + req.PayPal.Email)
	}

	meta := req.Metadata
	if meta == nil {
		meta = make(map[string]interface{})
	}
	if req.ProviderRef != "" {
		meta["squareVerified"] = true
	}
	pm.Metadata = meta

	if err := pm.Create(); err != nil {
		log.Error("Failed to create payment method: %v", err, c)
		return http.Fail(c, 500, "failed to create payment method", err)
	}

	return c.JSON(201, paymentMethodResponse(pm))
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

	return c.JSON(200, paymentMethodResponse(pm))
}

// ListPaymentMethods lists payment methods for a customer.
//
//	GET /v1/billing/methods?customerId=...&type=...
func ListPaymentMethods(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	rootKey := db.NewKey("synckey", "", 1, nil)
	methods := make([]*paymentmethod.PaymentMethod, 0)
	q := paymentmethod.Query(db).Ancestor(rootKey)

	// Intra-org IDOR guard (#43a, enumeration). A non-privileged org member may
	// only ever list its OWN subject's methods. EdgeAuth pins a PRESENT
	// customerId/user query param to the org slug, but an ABSENT filter would
	// return EVERY method in the namespace (incl. any service-token-created
	// per-user records) — so force the subject filter for non-privileged callers,
	// failing closed when no subject resolves. Privileged callers (service token /
	// admin / global admin) keep the explicit client-supplied filter.
	if isPrivilegedBillingCaller(c) {
		if customerId := c.Query("customerId"); customerId != "" {
			q = q.Filter("CustomerId=", customerId)
		} else if user := c.Query("user"); user != "" {
			q = q.Filter("CustomerId=", user)
		}
	} else if subject := orgBillingKey(c); subject != "" {
		q = q.Filter("CustomerId=", subject)
	} else {
		return c.JSON(200, []map[string]interface{}{})
	}
	if pmType := c.Query("type"); pmType != "" {
		q = q.Filter("Type=", pmType)
	}

	iter := q.Order("-Created").Run()
	for {
		pm := paymentmethod.New(db)
		if _, err := iter.Next(pm); err != nil {
			break
		}
		methods = append(methods, pm)
	}

	healPaymentMethods(c, org, methods)

	results := make([]map[string]interface{}, len(methods))
	for i, pm := range methods {
		results[i] = paymentMethodResponse(pm)
	}
	return c.JSON(200, results)
}

// healPaymentMethods backfills brand/last4/expiry onto vaulted rows that were
// saved before the vault reported card facts, so a customer can tell their
// saved cards apart. KMS-hydrated per-org processor, best-effort — see heal.
func healPaymentMethods(c *zip.Ctx, org *organization.Organization, methods []*paymentmethod.PaymentMethod) {
	if len(methods) == 0 {
		return
	}
	if v := c.Locals("kms"); v != nil {
		if kmsClient, ok := v.(*kms.CachedClient); ok {
			if err := kms.Hydrate(kmsClient, org); err != nil {
				log.Error("KMS hydration failed for org %q: %v", org.Name, err, c)
			}
		}
	}
	heal(c.Context(), payment.ProcessorsForOrg(org), methods)
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

	return c.JSON(200, paymentMethodResponse(pm))
}

// DetachPaymentMethod detaches (soft-deletes) a payment method.
//
//	DELETE /v1/billing/methods/:id
func DetachPaymentMethod(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	pm := paymentmethod.New(db)
	if err := pm.GetById(c.Param("id")); err != nil {
		return http.Fail(c, 404, "payment method not found", err)
	}

	// Intra-org IDOR guard (#43a, per-user). Detaching another subject's card
	// (soft-delete + Square card-on-file removal) breaks their saved-card charges
	// and auto-recharge — a cross-subject mutation the unpinned :id can reach in
	// the caller's namespace. Non-privileged callers may detach only their own; 404.
	if !callerMayReachBillingSubject(c, pm.CustomerId, pm.UserId) {
		return http.Fail(c, 404, "payment method not found", nil)
	}

	// Best-effort: detach the card-on-file from Square so its vault stays in
	// sync. Non-fatal — the local record is still soft-deleted on failure.
	if pm.Metadata != nil {
		custID, _ := pm.Metadata["squareCustomerId"].(string)
		cardID, _ := pm.Metadata["squareCardId"].(string)
		if custID != "" && cardID != "" {
			if v := c.Locals("kms"); v != nil {
				if kmsClient, ok := v.(*kms.CachedClient); ok {
					if err := kms.Hydrate(kmsClient, org); err != nil {
						log.Error("KMS hydration failed for org %q: %v", org.Name, err, c)
					}
				}
			}
			reg := payment.ProcessorsForOrg(org)
			if cp, ok := squareCustomerProcessorFrom(reg); ok {
				if err := cp.RemovePaymentMethod(c.Context(), custID, cardID); err != nil {
					log.Warn("Failed to remove Square card %s for customer %s: %v", cardID, custID, err)
				}
			}
		}
	}

	if err := pm.Delete(); err != nil {
		log.Error("Failed to detach payment method: %v", err, c)
		return http.Fail(c, 500, "failed to detach payment method", err)
	}

	return c.JSON(200, map[string]any{"deleted": true, "id": pm.Id()})
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

	return c.JSON(200, paymentMethodResponse(pm))
}

func paymentMethodResponse(pm *paymentmethod.PaymentMethod) map[string]interface{} {
	resp := map[string]interface{}{
		"id":         pm.Id(),
		"customerId": pm.CustomerId,
		"type":       pm.Type,
		"isDefault":  pm.IsDefault,
		"created":    pm.Created,
	}
	if pm.Name != "" {
		resp["name"] = pm.Name
	}
	if pm.Card != nil {
		resp["card"] = pm.Card
	}
	if pm.BankAccount != nil {
		resp["bankAccount"] = pm.BankAccount
	}
	if pm.Crypto != nil {
		resp["crypto"] = pm.Crypto
	}
	if pm.Wire != nil {
		resp["wire"] = pm.Wire
	}
	if pm.PayPal != nil {
		resp["paypal"] = pm.PayPal
	}
	if pm.BillingAddress != nil {
		resp["billingAddress"] = pm.BillingAddress
	}
	if pm.ProviderRef != "" {
		resp["providerRef"] = pm.ProviderRef
	}
	if pm.Metadata != nil {
		resp["metadata"] = pm.Metadata
	}
	return resp
}
