package config

import "path/filepath"

// Default settings
func Defaults() *Config {
	config := new(Config)

	config.Protocol = "//" // Protocol relative
	config.Hosts = make(map[string]string, 10)
	config.Prefixes = make(map[string]string, 10)
	config.RootDir, _ = filepath.Abs(cwd + "/../..")
	config.SiteTitle = "Hanzo"

	config.Fee = 0.05

	config.Secret = "change-me-in-production"
	config.SessionName = "session"

	config.DemoMode = demoMode

	config.Paypal.Email = "dev@hanzo.ai"
	config.Paypal.Api = "https://svcs.sandbox.paypal.com"
	config.Paypal.IpnUrl = "https://api.staging.hanzo.io/paypal/ipn/"
	config.Paypal.PaypalIpnUrl = "https://www.sandbox.paypal.com/cgi-bin/webscr"

	config.Stripe.DevelopmentClientId = "ca_REDACTED"
	config.Stripe.ProductionClientId = "ca_REDACTED"

	config.Stripe.TestSecretKey = ""
	config.Stripe.TestPublishableKey = "pk_test_REDACTED"
	config.Stripe.LiveSecretKey = ""
	config.Stripe.LivePublishablKey = "pk_live_REDACTED"

	config.Mandrill.FromName = "Hanzo"
	config.Mandrill.FromEmail = "noreply@hanzo.io"

	config.Netlify.BaseUrl = "https://api.netlify.com/api/v1"
	config.Netlify.ClientId = ""
	config.Netlify.Secret = ""

	config.Cloudflare.Email = "dev@hanzo.ai"
	config.Cloudflare.Key = ""
	config.Cloudflare.ZoneId = "hanzo.io"

	return config
}
