package billing

import (
	"context"
	"os"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/checkout"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/secrets"
	"github.com/hanzoai/commerce/thirdparty/kms"
	"github.com/hanzoai/commerce/types/integration"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// wireInstructionsResponse is the shape the pay SPA's WireInstructionsCard
// renders (pay/src/lib/api.ts WireInstructions). Field names are that contract.
type wireInstructionsResponse struct {
	BankName      string `json:"bankName"`
	BankAddress   string `json:"bankAddress,omitempty"`
	AccountNumber string `json:"accountNumber"`
	RoutingNumber string `json:"routingNumber,omitempty"`
	SwiftCode     string `json:"swiftCode,omitempty"`
	IBAN          string `json:"iban,omitempty"`
	AccountName   string `json:"accountName"`
	Memo          string `json:"memo,omitempty"`
	Reference     string `json:"reference"`
}

// GetWireInstructions returns the RECEIVING bank details for a wire top-up —
// the serving brand's own account (hanzo for pay.hanzo.ai, lux for
// pay.lux.network), never the caller's. The caller's identity goes into the
// payment reference so an arriving wire names who it credits; ops settle it
// with the admin wire/credit verb once the bank confirms receipt. No balance
// is ever minted here.
//
// Bank details live in KMS at /tenants/<brand>/wire/WIRE_* and hydrate onto
// the org row; an org without them answers 503, never a half-empty form.
//
//	GET /v1/billing/wire
func GetWireInstructions(c *zip.Ctx) error {
	payer := userBillingKey(c)
	if payer == "" {
		return jsonhttp.Fail(c, 401, "Authentication required", nil)
	}

	// The receiving side is a property of the HOST the customer is paying on,
	// resolved by the same brand table the tenant endpoint uses.
	slug := checkout.BrandSlugForHost(checkout.RequestHost(c))
	recv := organization.New(datastore.New(c.Context()))
	// Get reports a miss as (false, nil) — check found, not just err.
	if found, err := recv.Query().Filter("Name=", slug).Get(); err != nil || !found {
		return jsonhttp.Fail(c, 503, "Wire transfer not configured", err)
	}

	if kmsClient, ok := c.Locals("kms").(*kms.CachedClient); ok {
		if err := kms.Hydrate(kmsClient, recv); err != nil {
			log.Error("KMS hydration failed for org %q: %v", recv.Name, err, c)
		}
	}

	w := recv.Wire
	// Per-org KMS hydration only runs under KMS_ENABLED, which the co-resident
	// deployment does not set — so the org row is empty there and the rail 503s
	// however carefully the details were stored. Ask the HOST instead: it holds
	// an in-process KMS handle already. A per-org row still wins when present.
	if w.AccountNumber == "" && w.IBAN == "" {
		w = wireFromHost(c.Context())
	}
	if w.AccountNumber == "" && w.IBAN == "" {
		return jsonhttp.Fail(c, 503, "Wire transfer not configured", nil)
	}

	return c.JSON(200, wireInstructionsResponse{
		BankName:      w.BankName,
		BankAddress:   w.BankAddress,
		AccountNumber: w.AccountNumber,
		RoutingNumber: w.RoutingNumber,
		SwiftCode:     w.SWIFT,
		IBAN:          w.IBAN,
		AccountName:   w.AccountHolder,
		// BOTH carry the payer reference, and that is the point: banks label the
		// field differently (memo / reference / "message to beneficiary"), and a
		// customer types the value into whichever box their bank shows. Memo used
		// to carry the ORG's static WIRE_REFERENCE, which is unset — so the field
		// that links the money to an account rendered EMPTY, and an arriving wire
		// would have to be reconciled by hand from the amount and the sender name.
		Memo:      wireReference(payer),
		Reference: wireReference(payer),
	})
}

// wireReference renders the payer's billing key as a bank-memo-safe payment
// reference ("hanzo/z" → "TOPUP-HANZO-Z"). Wire memo fields routinely strip
// punctuation, so only [A-Z0-9-] survives the trip and the reference must be
// recoverable from that alphabet alone.
func wireReference(payer string) string {
	var b strings.Builder
	b.WriteString("TOPUP-")
	for _, r := range strings.ToUpper(payer) {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}

// wireFromHost reads the receiving-bank details from the HOST'S SECRET PLANE —
// in-process, by reference, no env and no HTTP.
//
// This replaced an env fan-out (KMS -> a k8s Secret -> WIRE_* on the pod ->
// os.Getenv). That chain worked, and it was three places for one value to go
// stale plus a pod restart to pick up a rotation, for a value the host already
// held. Every other plugin asks its host; so does this.
//
// The refs are the same coordinates the per-org hydrator uses, so one bank
// lives at one address whichever path reads it. os.Getenv survives ONLY as the
// last fallback for a standalone commerce with no host to ask.
func wireFromHost(ctx context.Context) integration.WireTransfer {
	get := func(name string) string {
		if v := secrets.String(ctx, "/tenants/hanzo/wire/"+name); v != "" {
			return v
		}
		return strings.TrimSpace(os.Getenv(name))
	}
	return integration.WireTransfer{
		BankName:      get("WIRE_BANK_NAME"),
		AccountHolder: get("WIRE_ACCOUNT_HOLDER"),
		AccountNumber: get("WIRE_ACCOUNT_NUMBER"),
		RoutingNumber: get("WIRE_ROUTING_NUMBER"),
		SWIFT:         get("WIRE_SWIFT"),
		IBAN:          get("WIRE_IBAN"),
		BankAddress:   get("WIRE_BANK_ADDRESS"),
		Reference:     get("WIRE_REFERENCE"),
		Instructions:  get("WIRE_INSTRUCTIONS"),
	}
}
