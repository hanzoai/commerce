package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/cryptopaymentintent"
	"github.com/hanzoai/commerce/payment/processor"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// The native crypto top-up rail: a per-payer MPC custody deposit address.
//
// Address generation is the MPC signer's (processor.MPC — Hanzo KMS control
// plane + MPC signing backend); commerce holds NO key material and can only
// REQUEST an address, exactly like the HUSD treasury split. Crediting the
// balance is the chain watcher's job on real confirmations — nothing here
// mints, so a claimed deposit is worth nothing until the chain says otherwise.
//
// The supported chain/token set is served live from the processor
// (GET /v1/billing/crypto/options), so when the MPC service grows a chain
// (TON, XRP, …) it appears on every pay surface with no client change.

// cryptoDepositRequest is the wire body. There is deliberately NO subject
// field: the credited payer is the caller's own billing identity, so a body
// value cannot steer where a deposit lands.
type cryptoDepositRequest struct {
	Chain       string `json:"chain,omitempty"` // default "ethereum"
	Token       string `json:"token,omitempty"` // default "usdc"
	AmountCents int64  `json:"amountCents,omitempty"`
}

type cryptoDepositResponse struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	Chain          string `json:"chain"`
	Token          string `json:"token"`
	DepositAddress string `json:"depositAddress"`
	ExpiresAt      string `json:"expiresAt,omitempty"`
}

// cryptoProcessor resolves the ONE crypto custody processor (default MPC,
// override via COMMERCE_DEFAULT_CRYPTO_PROCESSOR upstream in the registry).
// Availability is probed with a short deadline so an unconfigured or wedged
// MPC endpoint answers 503 fast instead of hanging the request.
func cryptoProcessor(ctx context.Context) (processor.CryptoProcessor, error) {
	p, err := processor.Get(processor.MPC)
	if err != nil {
		return nil, err
	}
	cp, ok := p.(processor.CryptoProcessor)
	if !ok {
		return nil, fmt.Errorf("processor %s does not support crypto operations", processor.MPC)
	}
	probe, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if !cp.IsAvailable(probe) {
		return nil, fmt.Errorf("crypto custody service unavailable")
	}
	return cp, nil
}

// GetCryptoOptions lists the chains and tokens the custody rail accepts,
// straight from the live processor — the pay SPA renders its asset picker
// from this, never from a hardcoded list.
//
//	GET /v1/billing/crypto/options
func GetCryptoOptions(c *zip.Ctx) error {
	cp, err := cryptoProcessor(c.Context())
	if err != nil {
		return jsonhttp.Fail(c, 503, "Crypto deposits not configured", err)
	}
	currencies := cp.SupportedCurrencies()
	tokens := make([]string, 0, len(currencies))
	for _, cur := range currencies {
		tokens = append(tokens, strings.ToLower(string(cur)))
	}
	return c.JSON(200, map[string]any{
		"chains": cp.SupportedChains(),
		"tokens": tokens,
	})
}

// CreateCryptoDeposit answers a custody deposit address for the caller. An
// open PENDING intent for the same (payer, chain, token) is reused — one
// payer, one live address per asset — so refreshing the page cannot spray
// MPC keygens or strand funds across addresses.
//
//	POST /v1/billing/crypto/deposit   { chain?, token?, amountCents? }
func CreateCryptoDeposit(c *zip.Ctx) error {
	payer := userBillingKey(c)
	if payer == "" {
		return jsonhttp.Fail(c, 401, "Authentication required", nil)
	}
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	var req cryptoDepositRequest
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return jsonhttp.Fail(c, 400, "invalid request body", err)
		}
	}
	chain := strings.ToLower(strings.TrimSpace(req.Chain))
	if chain == "" {
		chain = "ethereum"
	}
	token := strings.ToLower(strings.TrimSpace(req.Token))
	if token == "" {
		token = "usdc"
	}

	cp, err := cryptoProcessor(c.Context())
	if err != nil {
		return jsonhttp.Fail(c, 503, "Crypto deposits not configured", err)
	}
	if !supportedChain(cp, chain) {
		return jsonhttp.Fail(c, 400, fmt.Sprintf("unsupported chain %q", chain), nil)
	}

	// Reuse the payer's open intent for this asset before minting a new one.
	// Get reports a miss as (false, nil) — check found, not just err.
	existing := cryptopaymentintent.New(db)
	if found, err := existing.Query().
		Filter("CustomerRef=", payer).
		Filter("Chain=", chain).
		Filter("Token=", token).
		Filter("Status=", string(cryptopaymentintent.Pending)).
		Get(); err == nil && found && !existing.IsExpired() && existing.DepositAddress != "" {
		return c.JSON(200, toCryptoDepositResponse(existing))
	}

	address, err := cp.GenerateAddress(c.Context(), payer, chain)
	if err != nil {
		log.Error("crypto deposit address generation failed for %q on %s: %v", payer, chain, err, c)
		// 503, NOT 502 — and the difference is what the customer reads.
		// Cloudflare REPLACES an origin 502 with its own "Bad gateway"
		// interstitial, so the JSON below never reaches the browser: measured on
		// pay.hanzo.ai, the deposit call returned a text/html CF error page while
		// the identical call to the origin returned this message. 503 passes
		// through untouched (the wire rail's own "not configured" 503 proves it),
		// which is also the honest code: the custody signer is unavailable, we
		// are not a broken gateway.
		return jsonhttp.Fail(c, 503, "Crypto deposits are temporarily unavailable — the custody service is not accepting new addresses. Try again shortly or choose another way to pay.", err)
	}

	intent := cryptopaymentintent.New(db)
	intent.Amount = req.AmountCents
	intent.Currency = "usd"
	intent.Chain = cryptopaymentintent.Chain(chain)
	intent.Token = token
	intent.DepositAddress = address
	intent.CustomerRef = payer
	intent.Status = cryptopaymentintent.Pending
	// A top-up address is a standing destination, not a checkout hold — give it
	// a day rather than the model's 30-minute checkout default.
	intent.ExpiresAt = time.Now().Add(24 * time.Hour)
	intent.Defaults()
	if err := intent.Create(); err != nil {
		return jsonhttp.Fail(c, 500, "failed to record deposit intent", err)
	}

	return c.JSON(200, toCryptoDepositResponse(intent))
}

// GetCryptoDeposit reports an intent's state (pending → confirming →
// succeeded), scoped to the caller: another payer's intent id answers 404.
//
//	GET /v1/billing/crypto/deposit/:id
func GetCryptoDeposit(c *zip.Ctx) error {
	payer := userBillingKey(c)
	if payer == "" {
		return jsonhttp.Fail(c, 401, "Authentication required", nil)
	}
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	intent := cryptopaymentintent.New(db)
	if err := intent.GetById(c.Param("id")); err != nil {
		return jsonhttp.Fail(c, 404, "deposit not found", err)
	}
	if intent.CustomerRef != payer {
		return jsonhttp.Fail(c, 404, "deposit not found", nil)
	}
	return c.JSON(200, toCryptoDepositResponse(intent))
}

func toCryptoDepositResponse(i *cryptopaymentintent.CryptoPaymentIntent) cryptoDepositResponse {
	return cryptoDepositResponse{
		ID:             i.Id(),
		Status:         string(i.Status),
		Chain:          string(i.Chain),
		Token:          i.Token,
		DepositAddress: i.DepositAddress,
		ExpiresAt:      i.ExpiresAt.UTC().Format(time.RFC3339),
	}
}

func supportedChain(cp processor.CryptoProcessor, chain string) bool {
	for _, s := range cp.SupportedChains() {
		if s == chain {
			return true
		}
	}
	return false
}
