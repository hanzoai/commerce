package billing

import (
	"context"
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment"
	"github.com/hanzoai/commerce/payment/processor"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// PayPalPublicConfig is the PUBLIC PayPal JS SDK configuration returned alongside
// Square on GET /v1/billing/settings.
type PayPalPublicConfig struct {
	ClientId    string `json:"clientId"`
	Environment string `json:"environment"`
	Enabled     bool   `json:"enabled"`
}

// StripePublicConfig is the PUBLIC Stripe.js configuration returned alongside
// Square: the publishable key Stripe's card form mounts with. Enabled says some
// currency is paid through Stripe's card form here (PaymentConfig.Cards).
type StripePublicConfig struct {
	PublishableKey string `json:"publishableKey"`
	Environment    string `json:"environment"`
	Enabled        bool   `json:"enabled"`
}

// PaymentConfig is the PUBLIC payment configuration a browser needs to tokenize a
// card for an org: which provider, which application and location it tokenizes
// against, which environment that account lives in, whether the money is real,
// the public PayPal JS SDK config when enabled, and — per currency — whose card
// form takes the card (Cards: Square wherever Square can, Stripe where it
// cannot). Every field is safe to hand a client.
type PaymentConfig struct {
	Provider      string             `json:"provider"`
	ApplicationId string             `json:"applicationId"`
	LocationId    string             `json:"locationId"`
	Environment   string             `json:"environment"`
	Live          bool               `json:"live"`
	PayPal        PayPalPublicConfig `json:"paypal"`
	Stripe        StripePublicConfig `json:"stripe"`
	// Cards maps a currency ("usd") to the processor whose card form takes it
	// ("square" or "stripe"). A currency no card processor takes is absent.
	Cards map[string]string `json:"cards"`
	// Rail is the processor every NEW card payment goes through ("square" or
	// "stripe", rail.go). With the rail at stripe the page mounts Stripe for
	// every new card and offers no other rail; cards saved at Square still show,
	// and still renew what they pay for, but buy nothing new.
	Rail string `json:"rail"`
}

// ReadPaymentConfig resolves an org's public payment configuration.
//
// It takes the org as a value rather than reading it off a request because the
// browser is not the only asker: the public host→org tenant projection asks it,
// and a peer asks it over the internal plane. Two derivations of "which Square
// account is this" is a card tokenized against an account we cannot charge.
//
// Sandbox-vs-production comes from the SAME single authority as the charge path
// (org.TestMode / SQUARE_ENVIRONMENT) with the same KMS-then-env fallback, so the
// app id the browser tokenizes with always matches the env and access token
// commerce will vault and charge with.
//
// A nil org is not a failure — it is an org with no credentials of its own, which
// resolves to the deployment's public config. Nothing here can fail, so the
// answer is a value and no endpoint has an error to invent a status for.
//
// Cards is the selection rule's own answer (processor.Registry.Cards) for this
// org's processors with no buyer country, which this read does not know; the
// checkout's GET /v1/commerce/org answers the same rule for the caller's country.
func ReadPaymentConfig(ctx context.Context, org *organization.Organization) *PaymentConfig {
	sq := payment.SquarePublicConfig(org)
	pp := payment.PayPalPublicConfig(org)
	st := payment.StripePublicConfig(org)
	cards := map[string]string{}
	for cur, t := range processorsForOrg(orgOrZero(org)).Cards(ctx, "") {
		cards[string(cur)] = string(t)
	}
	rail := Rail(ctx)
	if rail == processor.Stripe {
		// One rail: every currency Stripe takes is Stripe's, and only Stripe's.
		cards = map[string]string{}
		if st.PublishableKey != "" {
			if p, err := processorsForOrg(orgOrZero(org)).Get(processor.Stripe); err == nil && p != nil {
				for _, cur := range p.SupportedCurrencies() {
					cards[string(cur)] = string(processor.Stripe)
				}
			}
		}
	}
	return &PaymentConfig{
		Rail: string(rail),
		Stripe: StripePublicConfig{
			PublishableKey: st.PublishableKey,
			Environment:    st.Environment,
			Enabled:        st.PublishableKey != "" && served(cards, processor.Stripe),
		},
		Cards:         cards,
		Provider:      string(rail),
		ApplicationId: sq.ApplicationID,
		LocationId:    sq.LocationID,
		Environment:   sq.Environment,
		Live:          sq.Live,
		PayPal: PayPalPublicConfig{
			ClientId:    pp.ClientID,
			Environment: pp.Environment,
			Enabled:     pp.ClientID != "" && !processor.DisabledByPolicy(processor.PayPal) && opens(ctx, processor.PayPal),
		},
	}
}

// GetPaymentConfig returns the PUBLIC Square config (application id, location id,
// environment) the browser's Web Payments SDK must use to tokenize a card for
// THIS org. All values are public (safe to expose to the client).
//
//	GET /v1/billing/settings
func GetPaymentConfig(c *zip.Ctx) error {
	// #146 class: resolve the org nil-safely. On the co-resident embed path there may be
	// no "organization" local; ReadPaymentConfig handles a nil org (env-fallback public
	// config), so a missing org yields the honest public config — never a nil-deref panic (502).
	org, _ := middleware.GetOrganizationOK(c)

	// NOTE: do NOT KMS-hydrate here. payment-config is called on dialog-open and
	// gates the card field mount, so it must be fast; a KMS round-trip can hang
	// when KMS is slow/unavailable and freeze the UI. Cloud orgs have no per-org
	// Square creds anyway — the env fallback supplies the public app id.
	return c.JSON(200, ReadPaymentConfig(c.Context(), org))
}

type testModeRequest struct {
	TestMode bool `json:"testMode"`
}

// Mode is which money an org is transacting: the org it names, and whether its
// transactions are test or live.
type Mode struct {
	OrgId    string `json:"orgId"`
	OrgName  string `json:"orgName"`
	Live     bool   `json:"live"`
	TestMode bool   `json:"testMode"`
}

// SetTestMode moves an org between test and live money and answers with the mode
// it now holds.
//
// It takes the org and the flag as values rather than reading them off a request
// because the mode is read back by everything that charges — a peer on the
// internal plane asks the same question — and a mode set one way and read another
// is how a sandbox charge lands on a live customer.
//
// WHO may move it is not decided here: the endpoint sits behind the money-mint bar,
// because a user who could put their own org in sandbox could dodge real charges.
func SetTestMode(ctx context.Context, org *organization.Organization, testMode bool) (*Mode, error) {
	if org == nil {
		return nil, errors.New("billing mode: no organization")
	}
	org.Live = !testMode
	if err := org.Update(); err != nil {
		return nil, err
	}
	return &Mode{OrgId: org.Id(), OrgName: org.Name, Live: !org.TestMode(), TestMode: org.TestMode()}, nil
}

// SetOrgTestMode toggles the org's live flag (org.Live) and its test-mode view.
// org.Live marks transactions Test=true and is the FALLBACK Square-environment
// signal: when the deployment does NOT set SQUARE_ENVIRONMENT, a test org uses
// Square sandbox and a live org uses production (via org.TestMode).
// When the deployment DOES set SQUARE_ENVIRONMENT (the per-env authority:
// mainnet=production, testnet/devnet=sandbox), that env governs which Square
// environment is charged regardless of this flag — on mainnet there is no
// sandbox charge. Admin-only — a user must not be able to move their own org to
// sandbox to dodge real charges.
//
//	POST /v1/billing/mode   { testMode: bool }
func SetOrgTestMode(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)

	var req testModeRequest
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}

	mode, err := SetTestMode(c.Context(), org, req.TestMode)
	if err != nil {
		log.Error("Failed to set test mode for org %q: %v", org.Name, err, c)
		return jsonhttp.Fail(c, 500, "failed to set test mode", err)
	}

	return c.JSON(200, mode)
}

// orgOrZero is org, or the zero org a nil caller stands for (the deployment's
// env-fallback credentials).
func orgOrZero(org *organization.Organization) *organization.Organization {
	if org == nil {
		return &organization.Organization{}
	}
	return org
}

// served reports whether any currency in cards is taken by processor t.
func served(cards map[string]string, t processor.ProcessorType) bool {
	for _, v := range cards {
		if v == string(t) {
			return true
		}
	}
	return false
}
