package billing

import (
	"context"
	"errors"
	"fmt"
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
// rail has, so a caller outside this module answers the same 503 the door does
// without knowing anything about how the details are stored.
func IsWireUnconfigured(err error) bool { return errors.Is(err, errNoWire) }

// WireFor is the whole wire read for a customer paying on a given host: which
// brand's bank they are sending to, that brand's org row, its secrets, and the
// reference that names them when the money lands.
//
// The host→brand reduction lives HERE rather than at the caller because the
// brand table is this module's; a caller doing it would keep a second copy of
// which hostnames are whose, and the two would drift the first time a domain is
// added. Everything a request holds that this needs — the host string, the
// caller's billing key, the host's KMS handle — arrives as a value, so a caller
// with no request and no datastore can still ask.
//
// kmsClient may be nil: per-org hydration only runs where KMS is enabled, and
// where it is not, the host's secret plane below is the answer rather than an
// error.
func WireFor(ctx context.Context, host, payer string, kmsClient *kms.CachedClient) (WireInstructions, error) {
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

	if kmsClient != nil {
		// Not fatal: the host's own secret plane is the fallback, and refusing a
		// configured rail because one hydration hiccupped would cost a top-up.
		if err := kms.Hydrate(kmsClient, recv); err != nil {
			log.Error("KMS hydration failed for org %q: %v", recv.Name, err)
		}
	}

	return GetWireInstructions(ctx, recv, brand, payer)
}

// GetWireInstructions assembles the RECEIVING bank details for a wire top-up —
// the serving brand's own account (hanzo for pay.hanzo.ai, lux for
// pay.lux.network), never the caller's. The caller's identity goes into the
// payment reference so an arriving wire names who it credits; ops settle it with
// the admin wire/credit verb once the bank confirms receipt. No balance is ever
// minted here.
//
// It takes the RESOLVED receiving org rather than looking one up, so a caller
// that already holds the row — WireFor, having just hydrated it — does not read
// it twice, and recv is allowed to be nil: that is not an error but the ordinary
// state of a deployment whose bank lives only in the host's secret plane.
//
// The payer must be named. The reference is the only thing that links arriving
// money to an account, so an anonymous one would be a destination that credits
// nobody — the exact failure a wire reference exists to prevent.
func GetWireInstructions(ctx context.Context, recv *organization.Organization, brand, payer string) (WireInstructions, error) {
	if payer == "" {
		return WireInstructions{}, fmt.Errorf("%w: no payer, so an arriving wire would name nobody", errNoWire)
	}

	var w integration.WireTransfer
	if recv != nil {
		w = recv.Wire
	}
	// Per-org KMS hydration only runs under KMS_ENABLED, which the co-resident
	// deployment does not set — so the org row is empty there and the rail 503s
	// however carefully the details were stored. Ask the HOST instead: it holds
	// an in-process KMS handle already. A per-org row still wins when present.
	//
	// The BRAND decides whose bank this is, so it decides which org's secrets to
	// read — the same slug that selected the receiving org row.
	if w.AccountNumber == "" && w.IBAN == "" {
		w = wireFromHost(ctx, brand)
	}
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

// GetBillingWireInstructions is the customer's door onto those details. It
// resolves the two things only a request knows — who is asking, and which host
// they are paying on — and hands the host's KMS handle to the read.
//
//	GET /v1/billing/wire
func GetBillingWireInstructions(c *zip.Ctx) error {
	payer := userBillingKey(c)
	if payer == "" {
		return jsonhttp.Fail(c, 401, "Authentication required", nil)
	}
	kmsClient, _ := c.Locals("kms").(*kms.CachedClient)

	out, err := WireFor(c.Context(), checkout.RequestHost(c), payer, kmsClient)
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

// wireFromHost reads the receiving-bank details from the HOST'S SECRET PLANE —
// in-process, by reference, no env and no HTTP.
//
// This replaced an env fan-out (KMS -> a k8s Secret -> WIRE_* on the pod ->
// os.Getenv). That chain worked, and it was three places for one value to go
// stale plus a pod restart to pick up a rotation, for a value the host already
// held. Every other plugin asks its host; so does this.
//
// THE ADDRESS IS `/orgs/<org>/wire/<NAME>`, which is the host's own convention
// and not a spelling invented here. cloud writes and reads every in-process KMS
// secret under `/orgs/{org}/…` — apps/destinations builds
// "/orgs/"+org+"/destinations/"+platform, apps/integrations the same for
// integrations, credz the same for service credentials — and its REST surface
// folds every write under that prefix from the validated org claim, because the
// org is the tenant boundary and belongs in the key.
//
// This read used `/tenants/hanzo/wire/<NAME>`, which nothing writes. Both doors
// hit the same store keyed by (path, name, env), so an address only one of them
// spells is a read that can never hit: the rail answered "Wire transfer not
// configured" no matter how carefully the details were stored, and would have
// gone on doing so silently. The org also stops being hardcoded — the brand
// serving the page decides whose bank is shown, so it decides whose secrets are
// read, and pay.lux.network resolves lux's bank rather than Hanzo's.
//
// os.Getenv survives ONLY as the last fallback for a standalone commerce with
// no host to ask.
func wireFromHost(ctx context.Context, org string) integration.WireTransfer {
	org = strings.TrimSpace(org)
	get := func(name string) string {
		if org != "" {
			if v := secrets.String(ctx, "/orgs/"+org+"/wire/"+name); v != "" {
				return v
			}
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
