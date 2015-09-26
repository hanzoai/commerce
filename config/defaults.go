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

	config.Paypal.PaypalApplicationId = "APP-80W284485P519543T"
	config.Paypal.PaypalSecurityUserId = "paypal_api1.verus.io"
	config.Paypal.PaypalSecurityPassword = "EH4HZWXCWXVDYWM2"
	config.Paypal.PaypalSecuritySignature = "AJd-SFo6hKDOAw2o1mufYejLBcKvAMX-QHZ9..uLkFX45mnUulajOXBJ"
	config.Paypal.ParallelPaymentsUrl = "https://svcs.sandbox.paypal.com/AdaptivePayments/Pay "

	config.Stripe.DevelopmentClientId = "ca_53yyPzxlPsdAtzMEIuS2mXYDp4FFXLmm"
	config.Stripe.ProductionClientId = "ca_53yyRUNpMtTRUgMlVlLAM3vllY1AVybU"

	config.Stripe.TestSecretKey = ""
	config.Stripe.TestPublishableKey = "pk_test_ucSTeAAtkSXVEg713ir40UhX"
	config.Stripe.LiveSecretKey = ""
	config.Stripe.LivePublishablKey = "pk_live_APr2mdiUblcOO4c2qTeyQ3hq"

	config.Mandrill.FromName = "Crowdstart"
	config.Mandrill.FromEmail = "noreply@crowdstart.com"

	config.Redis.Url = "pub-redis-19324.us-central1-1-1.gce.garantiadata.com:19324"
	config.Redis.Password = ""

	return config
}
