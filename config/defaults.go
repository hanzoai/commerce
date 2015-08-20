package config

import "path/filepath"

// Default settings
func Defaults() *Config {
	config := new(Config)

	config.Protocol = "//" // Protocol relative
	config.Hosts = make(map[string]string, 10)
	config.Prefixes = make(map[string]string, 10)
	config.RootDir, _ = filepath.Abs(cwd + "/../..")
	config.SiteTitle = "SKULLY"

	config.Secret = "change-me-in-production"
	config.SessionName = "session"

	config.DemoMode = demoMode

	config.Stripe.DevelopmentClientId = "ca_REDACTED"
	config.Stripe.ProductionClientId = "ca_REDACTED"

	config.Stripe.TestSecretKey = ""
	config.Stripe.TestPublishableKey = "pk_test_REDACTED"
	config.Stripe.LiveSecretKey = ""
	config.Stripe.LivePublishablKey = "pk_live_REDACTED"

	config.Mandrill.FromName = "Crowdstart"
	config.Mandrill.FromEmail = "noreply@crowdstart.com"

	config.Redis.Url = "http://cs-analytics-001.kc6goy.0001.usw1.cache.amazonaws.com"
	config.Redis.Password = ""

	return config
}
