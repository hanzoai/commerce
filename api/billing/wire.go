package billing

import (
	"os"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/checkout"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/organization"
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
	// ENV FALLBACK, the same shape Square's public config already uses. Per-org
	// KMS hydration only runs when KMS_ENABLED is set, and it is not set on the
	// co-resident deployment — so the org row's wire fields are empty there and
	// the rail 503s however carefully the secrets were stored. The deployment
	// carries them as WIRE_* instead (synced from KMS into commerce-secrets, the
	// path that demonstrably works), and a per-org row still wins when present.
	if w.AccountNumber == "" && w.IBAN == "" {
		w = wireFromEnv()
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
		Memo:          wireReference(payer),
		Reference:     wireReference(payer),
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

// wireFromEnv reads the deployment's receiving-bank details. Every field is
// optional except an account identifier — without one there is nowhere to send
// money, which is the single condition the caller treats as "not configured".
func wireFromEnv() integration.WireTransfer {
	return integration.WireTransfer{
		BankName:      os.Getenv("WIRE_BANK_NAME"),
		AccountHolder: os.Getenv("WIRE_ACCOUNT_HOLDER"),
		AccountNumber: os.Getenv("WIRE_ACCOUNT_NUMBER"),
		RoutingNumber: os.Getenv("WIRE_ROUTING_NUMBER"),
		SWIFT:         os.Getenv("WIRE_SWIFT"),
		IBAN:          os.Getenv("WIRE_IBAN"),
		BankAddress:   os.Getenv("WIRE_BANK_ADDRESS"),
		Reference:     os.Getenv("WIRE_REFERENCE"),
		Instructions:  os.Getenv("WIRE_INSTRUCTIONS"),
	}
}
