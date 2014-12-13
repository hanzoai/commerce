package config

// Staging Settings
func Staging() *Config {
	config := Production()

	config.IsProduction = false
	config.IsStaging = true

	config.Hosts["default"] = "default-dot-crowdstart-staging.appspot.com"
	config.Hosts["api"] = "api-dot-crowdstart-staging.appspot.com"
	config.Hosts["checkout"] = "checkout-dot-crowdstart-staging.appspot.com"
	config.Hosts["platform"] = "platform-dot-crowdstart-staging.appspot.com"
	config.Hosts["preorder"] = "preorder-dot-crowdstart-staging.appspot.com"
	config.Hosts["store"] = "store-dot-crowdstart-staging.appspot.com"

	config.StaticUrl = "//static-dot-crowdstart-staging.appspot.com"

	config.Stripe.ClientId = "ca_53yyPzxlPsdAtzMEIuS2mXYDp4FFXLmm"
	config.Stripe.APIKey = "pk_test_ucSTeAAtkSXVEg713ir40UhX"
	config.Stripe.APISecret = ""
	config.Stripe.RedirectURL = "https:" + config.UrlFor("platform", "/stripe/callback")
	config.Stripe.WebhookURL = "https:" + config.UrlFor("platform", "/stripe/hook")

	return config
}
