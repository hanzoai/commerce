package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/checkout"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// WireInstructions is the shape the pay SPA's WireInstructionsCard renders
// (pay/src/lib/api.ts WireInstructions). Field names are that contract.
type WireInstructions struct {
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

// errNoWire is the rail's ONE refusal, and every failure on the way to a set of
// bank details wraps it: to the customer they are a single fact — there is
// nowhere to send the money. A half-empty form is not an alternative, since
// nobody can wire to three fields out of five.
var errNoWire = errors.New("wire: no receiving account for this brand")

// IsWireUnconfigured reports that refusal. It is the only error class the wire
// rail has, so a caller outside this module answers the same 503 the endpoint does
// without knowing anything about how the details are stored.
func IsWireUnconfigured(err error) bool { return errors.Is(err, errNoWire) }

// WireFor is the whole wire read for a customer paying on a given host: which
// brand's bank they are sending to, that brand's org row, its secrets, and the
// reference that names them when the money lands.
//
// The host→brand reduction lives HERE rather than at the caller because the
// brand table is this module's; a caller doing it would keep a second copy of
// which hostnames are whose, and the two would drift the first time a domain is
// added. Everything a request holds that this needs — the host string and the
// caller's billing key — arrives as a value, so a caller with no request can
// still ask.
func WireFor(ctx context.Context, host, payer string) (WireInstructions, error) {
	// The receiving side is a property of the HOST the customer is paying on,
	// resolved by the same brand table the tenant endpoint uses.
	brand := checkout.BrandSlugForHost(host)
	recv := organization.New(datastore.New(ctx))
	// Get reports a miss as (false, nil) — check found, not just err.
	found, err := recv.Query().Filter("Name=", brand).Get()
	if err != nil {
		return WireInstructions{}, fmt.Errorf("%w: reading the org serving %q: %s", errNoWire, brand, err)
	}
	if !found {
		return WireInstructions{}, fmt.Errorf("%w: no org serves %q", errNoWire, brand)
	}

	// The brand is the org the money is sent to, so its own bank details are read:
	// orgs/<brand>/wire/<NAME>, through the host's secret plane.
	payment.Hydrate(ctx, recv, "wire")
	return GetWireInstructions(recv, payer)
}

// GetWireInstructions assembles the RECEIVING bank details for a wire top-up —
// the serving brand's own account (hanzo for pay.hanzo.ai, lux for
// pay.lux.network), never the caller's. The caller's identity goes into the
// payment reference so an arriving wire names who it credits; ops settle it with
// the admin wire/credit verb once the bank confirms receipt. No balance is ever
// minted here.
//
// It takes the RESOLVED, hydrated receiving org rather than looking one up.
//
// The payer must be named. The reference is the only thing that links arriving
// money to an account, so an anonymous one would be a destination that credits
// nobody — the exact failure a wire reference exists to prevent.
func GetWireInstructions(recv *organization.Organization, payer string) (WireInstructions, error) {
	if payer == "" {
		return WireInstructions{}, fmt.Errorf("%w: no payer, so an arriving wire would name nobody", errNoWire)
	}
	if recv == nil {
		return WireInstructions{}, errNoWire
	}
	w := recv.Wire
	if w.AccountNumber == "" && w.IBAN == "" {
		return WireInstructions{}, errNoWire
	}

	return WireInstructions{
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
	}, nil
}

// GetBillingWireInstructions is the customer's endpoint for those details. It
// resolves the two things only a request knows — who is asking, and which host
// they are paying on — and hands them to the read.
//
//	GET /v1/billing/wire
func GetBillingWireInstructions(c *zip.Ctx) error {
	payer := userBillingKey(c)
	if payer == "" {
		return jsonhttp.Fail(c, 401, "Authentication required", nil)
	}
	// Forward carries the request's caller, which the host's KMS reads to admit
	// the brand's bank read; a raw handler's Context carries none.
	out, err := WireFor(c.Forward(), checkout.RequestHost(c), payer)
	if err != nil {
		return jsonhttp.Fail(c, 503, "Wire transfer not configured", err)
	}
	return c.JSON(200, out)
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
