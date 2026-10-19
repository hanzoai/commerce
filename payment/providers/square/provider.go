// Package square is commerce's one Square payment processor: the type every
// registry holds for processor.Square. processor.Global() holds one seeded from
// the deployment environment, which is what verifies webhooks; a per-org registry
// (payment.ProcessorsForOrg) holds one configured with the charging org's
// credentials.
//
// thirdparty/square is the Square API client underneath. It moves money and
// manages customers and cards; it does not verify webhooks and does not register
// itself anywhere. The lifecycle — configuration, availability, verification —
// is here.
package square

import (
	"context"
	"os"
	"strings"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	thirdparty "github.com/hanzoai/commerce/thirdparty/square"
)

// Config holds Square credentials and environment settings.
type Config struct {
	// ApplicationID is the Square Developer Dashboard application ID.
	// Required for the Web Payments SDK (browser-side tokenization).
	ApplicationID string
	// AccessToken is the server-side bearer that authenticates calls to the
	// Square API.
	AccessToken string
	// LocationID pins charges to a specific Square merchant location.
	LocationID string
	// WebhookSignatureKey is the signature key of the Square webhook
	// subscription; ValidateWebhook keys its HMAC with it.
	WebhookSignatureKey string
	// WebhookURL is the EXACT notification URL of that subscription. Square
	// signs HMAC over notificationURL+body, so this is part of the proof.
	WebhookURL string
	// Environment must be "sandbox" or "production".
	Environment string
	// Org is the org this Square account belongs to: a delivery it verifies is
	// applied to that org's books. Empty is the deployment's own account.
	Org string
}

// Provider implements processor.PaymentProcessor and
// processor.CustomerProcessor for Square, delegating every API call to one
// *thirdparty.SquareProcessor.
type Provider struct {
	*processor.BaseProcessor
	config Config
	inner  *thirdparty.SquareProcessor
}

func init() {
	// The global registry's Square slot, seeded from the deployment. It is the
	// sessionless verifier HandleProviderWebhook reaches with no tenant resolver
	// in front, so it is configured here or not at all.
	p := &Provider{
		BaseProcessor: processor.NewBaseProcessor(processor.Square, thirdparty.SquareSupportedCurrencies()),
	}
	p.Configure(configFromEnv())
	processor.Register(p)
}

// configFromEnv is the deployment's own Square account: SQUARE_ENVIRONMENT picks
// the sandbox or production credential set, and only that set
// (thirdparty.FromEnv), plus the webhook subscription's key and URL.
func configFromEnv() Config {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("SQUARE_ENVIRONMENT")))
	sandbox := thirdparty.SandboxEnv()
	creds := thirdparty.FromEnv(sandbox)
	if sandbox {
		env = "sandbox"
	} else if env == "" {
		env = "production"
	}
	return Config{
		ApplicationID:       creds.ApplicationID,
		AccessToken:         creds.AccessToken,
		LocationID:          creds.LocationID,
		WebhookSignatureKey: strings.TrimSpace(os.Getenv("SQUARE_WEBHOOK_SIGNATURE_KEY")),
		WebhookURL:          strings.TrimSpace(os.Getenv("SQUARE_WEBHOOK_URL")),
		Environment:         env,
	}
}

// NewProvider builds and configures a Provider in one shot.
func NewProvider(cfg Config) *Provider {
	p := &Provider{
		BaseProcessor: processor.NewBaseProcessor(processor.Square, thirdparty.SquareSupportedCurrencies()),
	}
	p.Configure(cfg)
	return p
}

// Configure replaces the provider's configuration. The webhook half is kept
// whether or not charge credentials are present; the API client exists only
// with an access token and a location.
func (p *Provider) Configure(cfg Config) {
	p.config = cfg
	if cfg.AccessToken == "" || cfg.LocationID == "" {
		p.inner = nil
		p.SetConfigured(false)
		return
	}
	p.inner = thirdparty.NewProcessor(thirdparty.Config{
		AccessToken: cfg.AccessToken,
		LocationID:  cfg.LocationID,
		Environment: cfg.Environment,
	})
	p.SetConfigured(true)
}

// Type returns the processor type.
func (p *Provider) Type() processor.ProcessorType {
	return processor.Square
}

// IsAvailable reports whether the provider can charge: an API client exists.
func (p *Provider) IsAvailable(ctx context.Context) bool {
	return p.inner != nil && p.config.AccessToken != "" && p.config.LocationID != ""
}

// SupportedCurrencies returns the currencies Square supports.
func (p *Provider) SupportedCurrencies() []currency.Type {
	return thirdparty.SquareSupportedCurrencies()
}

// ApplicationID returns the browser-facing application ID.
func (p *Provider) ApplicationID() string { return p.config.ApplicationID }

// LocationID returns the configured merchant location.
func (p *Provider) LocationID() string { return p.config.LocationID }

// Environment returns "sandbox" or "production".
func (p *Provider) Environment() string { return p.config.Environment }

var (
	_ processor.PaymentProcessor  = (*Provider)(nil)
	_ processor.CustomerProcessor = (*Provider)(nil)
)
