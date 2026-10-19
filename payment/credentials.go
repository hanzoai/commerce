// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package payment

import (
	"context"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/secrets"
)

// credential is one secret of a provider's credential set and the org field it
// fills.
type credential struct {
	provider string
	name     string
	set      func(org *organization.Organization, val string)
}

// Hydrate loads org's OWN payment credentials into its integration fields, read
// through the host's secret plane (package secrets) from orgs/<org>/<provider>/<NAME>.
// providers narrows the read to those credential sets; none reads every set.
//
// It is for the paths where org is the MERCHANT — the org that receives the
// money: a storefront's checkout, its subscriptions, the brand whose bank a wire
// is sent to. Hanzo's own billing (a customer paying the brand for credit or a
// plan) does not call it on the paying org, because the paying org's Square
// account is not where that money goes; that path charges the deployment's own
// account through ProcessorsForOrg's environment fallback.
//
// A secret that is absent leaves its field as it was. Nothing is cached: the
// store is the process next door, and a rotation lands on the next read.
func Hydrate(ctx context.Context, org *organization.Organization, providers ...string) {
	if org == nil || org.Name == "" {
		return
	}
	want := map[string]bool{}
	for _, p := range providers {
		want[p] = true
	}
	for _, c := range credentials {
		if len(want) > 0 && !want[c.provider] {
			continue
		}
		if v := secrets.Org(ctx, org.Name, c.provider+"/"+c.name); v != "" {
			c.set(org, v)
		}
	}
}

// credentials is every provider credential an org can hold, by the name it is
// stored under.
var credentials = []credential{
	// Stripe — Live
	{"stripe", "STRIPE_LIVE_ACCESS_TOKEN", func(o *organization.Organization, v string) { o.Stripe.Live.AccessToken = v }},
	{"stripe", "STRIPE_LIVE_PUBLISHABLE_KEY", func(o *organization.Organization, v string) { o.Stripe.Live.PublishableKey = v }},
	{"stripe", "STRIPE_LIVE_REFRESH_TOKEN", func(o *organization.Organization, v string) { o.Stripe.Live.RefreshToken = v }},
	{"stripe", "STRIPE_LIVE_USER_ID", func(o *organization.Organization, v string) { o.Stripe.Live.UserId = v }},
	// Stripe — Test
	{"stripe", "STRIPE_TEST_ACCESS_TOKEN", func(o *organization.Organization, v string) { o.Stripe.Test.AccessToken = v }},
	{"stripe", "STRIPE_TEST_PUBLISHABLE_KEY", func(o *organization.Organization, v string) { o.Stripe.Test.PublishableKey = v }},
	{"stripe", "STRIPE_TEST_REFRESH_TOKEN", func(o *organization.Organization, v string) { o.Stripe.Test.RefreshToken = v }},
	{"stripe", "STRIPE_TEST_USER_ID", func(o *organization.Organization, v string) { o.Stripe.Test.UserId = v }},

	// Square — Production
	{"square", "SQUARE_PRODUCTION_ACCESS_TOKEN", func(o *organization.Organization, v string) { o.Square.Production.AccessToken = v }},
	{"square", "SQUARE_PRODUCTION_LOCATION_ID", func(o *organization.Organization, v string) { o.Square.Production.LocationId = v }},
	{"square", "SQUARE_PRODUCTION_APPLICATION_ID", func(o *organization.Organization, v string) { o.Square.Production.ApplicationId = v }},
	// Square — Sandbox
	{"square", "SQUARE_SANDBOX_ACCESS_TOKEN", func(o *organization.Organization, v string) { o.Square.Sandbox.AccessToken = v }},
	{"square", "SQUARE_SANDBOX_LOCATION_ID", func(o *organization.Organization, v string) { o.Square.Sandbox.LocationId = v }},
	{"square", "SQUARE_SANDBOX_APPLICATION_ID", func(o *organization.Organization, v string) { o.Square.Sandbox.ApplicationId = v }},
	// Square — Webhook
	{"square", "SQUARE_WEBHOOK_SIGNATURE_KEY", func(o *organization.Organization, v string) { o.Square.WebhookSignatureKey = v }},
	{"square", "SQUARE_WEBHOOK_URL", func(o *organization.Organization, v string) { o.Square.WebhookURL = v }},

	// AuthorizeNet — Live
	{"authorizenet", "AUTHORIZENET_LIVE_LOGIN_ID", func(o *organization.Organization, v string) { o.AuthorizeNet.Live.LoginId = v }},
	{"authorizenet", "AUTHORIZENET_LIVE_TRANSACTION_KEY", func(o *organization.Organization, v string) { o.AuthorizeNet.Live.TransactionKey = v }},
	// AuthorizeNet — Sandbox
	{"authorizenet", "AUTHORIZENET_SANDBOX_LOGIN_ID", func(o *organization.Organization, v string) { o.AuthorizeNet.Sandbox.LoginId = v }},
	{"authorizenet", "AUTHORIZENET_SANDBOX_TRANSACTION_KEY", func(o *organization.Organization, v string) { o.AuthorizeNet.Sandbox.TransactionKey = v }},

	// PayPal — Live
	{"paypal", "PAYPAL_LIVE_EMAIL", func(o *organization.Organization, v string) { o.Paypal.Live.Email = v }},
	{"paypal", "PAYPAL_LIVE_SECURITY_USER_ID", func(o *organization.Organization, v string) { o.Paypal.Live.SecurityUserId = v }},
	{"paypal", "PAYPAL_LIVE_SECURITY_PASSWORD", func(o *organization.Organization, v string) { o.Paypal.Live.SecurityPassword = v }},
	{"paypal", "PAYPAL_LIVE_SECURITY_SIGNATURE", func(o *organization.Organization, v string) { o.Paypal.Live.SecuritySignature = v }},
	{"paypal", "PAYPAL_LIVE_APPLICATION_ID", func(o *organization.Organization, v string) { o.Paypal.Live.ApplicationId = v }},
	// PayPal — Test
	{"paypal", "PAYPAL_TEST_EMAIL", func(o *organization.Organization, v string) { o.Paypal.Test.Email = v }},
	{"paypal", "PAYPAL_TEST_SECURITY_USER_ID", func(o *organization.Organization, v string) { o.Paypal.Test.SecurityUserId = v }},
	{"paypal", "PAYPAL_TEST_SECURITY_PASSWORD", func(o *organization.Organization, v string) { o.Paypal.Test.SecurityPassword = v }},
	{"paypal", "PAYPAL_TEST_SECURITY_SIGNATURE", func(o *organization.Organization, v string) { o.Paypal.Test.SecuritySignature = v }},
	{"paypal", "PAYPAL_TEST_APPLICATION_ID", func(o *organization.Organization, v string) { o.Paypal.Test.ApplicationId = v }},

	// Adyen
	{"adyen", "ADYEN_API_KEY", func(o *organization.Organization, v string) { o.Adyen.APIKey = v }},
	{"adyen", "ADYEN_MERCHANT_ACCOUNT", func(o *organization.Organization, v string) { o.Adyen.MerchantAccount = v }},
	{"adyen", "ADYEN_HMAC_KEY", func(o *organization.Organization, v string) { o.Adyen.HMACKey = v }},
	{"adyen", "ADYEN_ENVIRONMENT", func(o *organization.Organization, v string) { o.Adyen.Environment = v }},
	{"adyen", "ADYEN_LIVE_URL_PREFIX", func(o *organization.Organization, v string) { o.Adyen.LiveURLPrefix = v }},

	// Braintree
	{"braintree", "BRAINTREE_MERCHANT_ID", func(o *organization.Organization, v string) { o.Braintree.MerchantID = v }},
	{"braintree", "BRAINTREE_PUBLIC_KEY", func(o *organization.Organization, v string) { o.Braintree.PublicKey = v }},
	{"braintree", "BRAINTREE_PRIVATE_KEY", func(o *organization.Organization, v string) { o.Braintree.PrivateKey = v }},
	{"braintree", "BRAINTREE_ENVIRONMENT", func(o *organization.Organization, v string) { o.Braintree.Environment = v }},

	// Recurly
	{"recurly", "RECURLY_API_KEY", func(o *organization.Organization, v string) { o.Recurly.APIKey = v }},
	{"recurly", "RECURLY_SUBDOMAIN", func(o *organization.Organization, v string) { o.Recurly.Subdomain = v }},

	// LemonSqueezy
	{"lemonsqueezy", "LEMONSQUEEZY_API_KEY", func(o *organization.Organization, v string) { o.LemonSqueezy.APIKey = v }},
	{"lemonsqueezy", "LEMONSQUEEZY_STORE_ID", func(o *organization.Organization, v string) { o.LemonSqueezy.StoreID = v }},
	{"lemonsqueezy", "LEMONSQUEEZY_WEBHOOK_SECRET", func(o *organization.Organization, v string) { o.LemonSqueezy.WebhookSecret = v }},
	{"lemonsqueezy", "LEMONSQUEEZY_DEFAULT_VARIANT_ID", func(o *organization.Organization, v string) { o.LemonSqueezy.DefaultVariantID = v }},

	// Wire Transfer
	{"wire", "WIRE_BANK_NAME", func(o *organization.Organization, v string) { o.Wire.BankName = v }},
	{"wire", "WIRE_ACCOUNT_HOLDER", func(o *organization.Organization, v string) { o.Wire.AccountHolder = v }},
	{"wire", "WIRE_ROUTING_NUMBER", func(o *organization.Organization, v string) { o.Wire.RoutingNumber = v }},
	{"wire", "WIRE_ACCOUNT_NUMBER", func(o *organization.Organization, v string) { o.Wire.AccountNumber = v }},
	{"wire", "WIRE_SWIFT", func(o *organization.Organization, v string) { o.Wire.SWIFT = v }},
	{"wire", "WIRE_IBAN", func(o *organization.Organization, v string) { o.Wire.IBAN = v }},
	{"wire", "WIRE_BANK_ADDRESS", func(o *organization.Organization, v string) { o.Wire.BankAddress = v }},
	{"wire", "WIRE_REFERENCE", func(o *organization.Organization, v string) { o.Wire.Reference = v }},
	{"wire", "WIRE_INSTRUCTIONS", func(o *organization.Organization, v string) { o.Wire.Instructions = v }},

	// Mercury
	{"mercury", "MERCURY_API_TOKEN", func(o *organization.Organization, v string) { o.Mercury.APIToken = v }},
	{"mercury", "MERCURY_WEBHOOK_SECRET", func(o *organization.Organization, v string) { o.Mercury.WebhookSecret = v }},
	{"mercury", "MERCURY_ACCOUNT_ID", func(o *organization.Organization, v string) { o.Mercury.AccountID = v }},
}
