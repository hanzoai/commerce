package kms

import (
	"strings"

	"github.com/hanzoai/commerce/models/organization"
)

// secretMapping maps a KMS secret name to a function that sets the value on the org.
type secretMapping struct {
	path string
	name string
	set  func(org *organization.Organization, val string)
}

// mappings returns all provider credential mappings for a given org name.
func mappings(orgName string) []secretMapping {
	stripe := "/tenants/" + orgName + "/stripe"
	square := "/tenants/" + orgName + "/square"
	authnet := "/tenants/" + orgName + "/authorizenet"
	paypal := "/tenants/" + orgName + "/paypal"

	return []secretMapping{
		// Stripe
		{stripe, "STRIPE_LIVE_ACCESS_TOKEN", func(o *organization.Organization, v string) { o.Stripe.Live.AccessToken = v }},
		{stripe, "STRIPE_TEST_ACCESS_TOKEN", func(o *organization.Organization, v string) { o.Stripe.Test.AccessToken = v }},
		{stripe, "STRIPE_PUBLISHABLE_KEY", func(o *organization.Organization, v string) { o.Stripe.PublishableKey = v }},

		// Square — Production
		{square, "SQUARE_PRODUCTION_ACCESS_TOKEN", func(o *organization.Organization, v string) { o.Square.Production.AccessToken = v }},
		{square, "SQUARE_PRODUCTION_LOCATION_ID", func(o *organization.Organization, v string) { o.Square.Production.LocationId = v }},
		{square, "SQUARE_PRODUCTION_APPLICATION_ID", func(o *organization.Organization, v string) { o.Square.Production.ApplicationId = v }},
		// Square — Sandbox
		{square, "SQUARE_SANDBOX_ACCESS_TOKEN", func(o *organization.Organization, v string) { o.Square.Sandbox.AccessToken = v }},
		{square, "SQUARE_SANDBOX_LOCATION_ID", func(o *organization.Organization, v string) { o.Square.Sandbox.LocationId = v }},
		{square, "SQUARE_SANDBOX_APPLICATION_ID", func(o *organization.Organization, v string) { o.Square.Sandbox.ApplicationId = v }},
		// Square — Webhook
		{square, "SQUARE_WEBHOOK_SIGNATURE_KEY", func(o *organization.Organization, v string) { o.Square.WebhookSignatureKey = v }},

		// AuthorizeNet — Live
		{authnet, "AUTHORIZENET_LIVE_LOGIN_ID", func(o *organization.Organization, v string) { o.AuthorizeNet.Live.LoginId = v }},
		{authnet, "AUTHORIZENET_LIVE_TRANSACTION_KEY", func(o *organization.Organization, v string) { o.AuthorizeNet.Live.TransactionKey = v }},
		// AuthorizeNet — Sandbox
		{authnet, "AUTHORIZENET_SANDBOX_LOGIN_ID", func(o *organization.Organization, v string) { o.AuthorizeNet.Sandbox.LoginId = v }},
		{authnet, "AUTHORIZENET_SANDBOX_TRANSACTION_KEY", func(o *organization.Organization, v string) { o.AuthorizeNet.Sandbox.TransactionKey = v }},

		// PayPal — Live
		{paypal, "PAYPAL_LIVE_EMAIL", func(o *organization.Organization, v string) { o.Paypal.Live.Email = v }},
		{paypal, "PAYPAL_LIVE_SECURITY_USER_ID", func(o *organization.Organization, v string) { o.Paypal.Live.SecurityUserId = v }},
		{paypal, "PAYPAL_LIVE_SECURITY_PASSWORD", func(o *organization.Organization, v string) { o.Paypal.Live.SecurityPassword = v }},
		{paypal, "PAYPAL_LIVE_SECURITY_SIGNATURE", func(o *organization.Organization, v string) { o.Paypal.Live.SecuritySignature = v }},
		{paypal, "PAYPAL_LIVE_APPLICATION_ID", func(o *organization.Organization, v string) { o.Paypal.Live.ApplicationId = v }},
		// PayPal — Test
		{paypal, "PAYPAL_TEST_EMAIL", func(o *organization.Organization, v string) { o.Paypal.Test.Email = v }},
		{paypal, "PAYPAL_TEST_SECURITY_USER_ID", func(o *organization.Organization, v string) { o.Paypal.Test.SecurityUserId = v }},
		{paypal, "PAYPAL_TEST_SECURITY_PASSWORD", func(o *organization.Organization, v string) { o.Paypal.Test.SecurityPassword = v }},
		{paypal, "PAYPAL_TEST_SECURITY_SIGNATURE", func(o *organization.Organization, v string) { o.Paypal.Test.SecuritySignature = v }},
		{paypal, "PAYPAL_TEST_APPLICATION_ID", func(o *organization.Organization, v string) { o.Paypal.Test.ApplicationId = v }},
	}
}

// Hydrate fetches all provider credentials from KMS and populates the org's
// integration fields. Missing secrets are silently skipped (not every org uses
// every provider). Only KMS communication failures are returned as errors.
func Hydrate(cc *CachedClient, org *organization.Organization) error {
	for _, m := range mappings(org.Name) {
		val, err := cc.GetSecret(m.path, m.name)
		if err != nil {
			// "not found" errors are expected — skip silently
			if strings.Contains(err.Error(), "status 404") || strings.Contains(err.Error(), "status 400") {
				continue
			}
			// Real communication failure
			return err
		}
		if val != "" {
			m.set(org, val)
		}
	}
	return nil
}
