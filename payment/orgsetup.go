// Package payment provides per-org payment processor configuration.
//
// The payment/processor registry holds global singleton processors registered at
// init() time. These singletons are NOT safe for multi-tenant use because
// credentials would be shared across orgs.
//
// ProcessorsForOrg creates a FRESH registry with per-org processor instances,
// each configured with credentials from the KMS-hydrated Organization model.
// Call kms.Hydrate(cc, org) before calling ProcessorsForOrg.
package payment

import (
	"os"
	"strings"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/providers/adyen"
	"github.com/hanzoai/commerce/payment/providers/braintree"
	"github.com/hanzoai/commerce/payment/providers/lemonsqueezy"
	"github.com/hanzoai/commerce/payment/providers/paypal"
	"github.com/hanzoai/commerce/payment/providers/recurly"
	"github.com/hanzoai/commerce/payment/providers/stripe"
	square "github.com/hanzoai/commerce/thirdparty/square"
)

// ProcessorsForOrg returns a processor registry configured with the given
// organization's payment credentials. Each processor is a fresh instance —
// no shared state with other orgs or the global singleton registry.
//
// Processors without credentials in the org are still registered but marked
// as unavailable (IsAvailable returns false).
func ProcessorsForOrg(org *organization.Organization) *processor.Registry {
	reg := processor.NewRegistry(processor.DefaultConfig())

	// Stripe
	sk := org.StripeToken()
	if sk != "" {
		reg.Register(stripe.NewProvider(stripe.Config{
			SecretKey:      sk,
			PublishableKey: org.Stripe.Live.PublishableKey,
			WebhookSecret:  "", // webhook secret is global, not per-org
		}))
	}

	// Square. Prefer the org's KMS-hydrated config; fall back to process env
	// vars when KMS is disabled or the org has no stored Square credentials.
	// This mirrors the checkout-session resolver (api/checkout/sessions.go),
	// which has always honored SQUARE_ACCESS_TOKEN / SQUARE_LOCATION_ID.
	{
		sqCfg := org.SquareConfig(!org.Live)
		token := sqCfg.AccessToken
		locationID := sqCfg.LocationId
		webhookKey := org.Square.WebhookSignatureKey
		env := "production"
		if !org.Live {
			env = "sandbox"
		}

		if token == "" {
			squareEnv := strings.ToLower(strings.TrimSpace(os.Getenv("SQUARE_ENVIRONMENT")))
			envSandbox := squareEnv == "sandbox" || squareEnv == "test"

			token = strings.TrimSpace(os.Getenv("SQUARE_ACCESS_TOKEN"))
			locationID = strings.TrimSpace(os.Getenv("SQUARE_LOCATION_ID"))
			if envSandbox {
				if t := strings.TrimSpace(os.Getenv("SQUARE_SANDBOX_ACCESS_TOKEN")); t != "" {
					token = t
				}
				if l := strings.TrimSpace(os.Getenv("SQUARE_SANDBOX_LOCATION_ID")); l != "" {
					locationID = l
				}
			}
			if webhookKey == "" {
				webhookKey = strings.TrimSpace(os.Getenv("SQUARE_WEBHOOK_SIGNATURE_KEY"))
			}
			if envSandbox {
				env = "sandbox"
			} else {
				env = "production"
			}
		}

		if token != "" && locationID != "" {
			reg.Register(square.NewProcessor(square.Config{
				AccessToken:   token,
				LocationID:    locationID,
				WebhookSecret: webhookKey,
				Environment:   env,
			}))
		}
	}

	// Adyen
	if org.Adyen.APIKey != "" {
		reg.Register(adyen.NewProvider(adyen.Config{
			APIKey:          org.Adyen.APIKey,
			MerchantAccount: org.Adyen.MerchantAccount,
			HMACKey:         org.Adyen.HMACKey,
			Environment:     adyen.Environment(org.Adyen.Environment),
			LiveURLPrefix:   org.Adyen.LiveURLPrefix,
		}))
	}

	// Braintree
	if org.Braintree.PublicKey != "" {
		reg.Register(braintree.NewProvider(braintree.Config{
			MerchantID:  org.Braintree.MerchantID,
			PublicKey:    org.Braintree.PublicKey,
			PrivateKey:  org.Braintree.PrivateKey,
			Environment: org.Braintree.Environment,
		}))
	}

	// PayPal — use v2 REST API client credentials
	// The legacy org.Paypal fields use Adaptive Payments (deprecated).
	// Map securityUserId → clientID, securityPassword → clientSecret.
	ppCreds := org.Paypal.Live
	ppSandbox := false
	if !org.Live {
		ppCreds = org.Paypal.Test
		ppSandbox = true
	}
	if ppCreds.SecurityUserId != "" {
		reg.Register(paypal.NewProvider(paypal.Config{
			ClientID:     ppCreds.SecurityUserId,
			ClientSecret: ppCreds.SecurityPassword,
			Sandbox:      ppSandbox,
		}))
	}

	// Recurly
	if org.Recurly.APIKey != "" {
		reg.Register(recurly.NewProvider(recurly.Config{
			APIKey:    org.Recurly.APIKey,
			Subdomain: org.Recurly.Subdomain,
		}))
	}

	// LemonSqueezy
	if org.LemonSqueezy.APIKey != "" {
		reg.Register(lemonsqueezy.NewProvider(lemonsqueezy.Config{
			APIKey:           org.LemonSqueezy.APIKey,
			StoreID:          org.LemonSqueezy.StoreID,
			WebhookSecret:    org.LemonSqueezy.WebhookSecret,
			DefaultVariantID: org.LemonSqueezy.DefaultVariantID,
		}))
	}

	return reg
}
