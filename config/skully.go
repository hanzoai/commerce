package config

// SKULLY Settings
func Skully() *Config {
	config := Production()

	config.Hosts["default"] = "static.skullysystems.com"
	config.Hosts["api"] = "invalid.skullysystems.com" // Setting platform to API temporarily.
	config.Hosts["checkout"] = "secure.skullysystems.com"
	config.Hosts["platform"] = "api.skullysystems.com"
	config.Hosts["preorder"] = "preorder.skullysystems.com"
	config.Hosts["store"] = "store.skullysystems.com"

	config.CookieDomain = "skullysystems.com"

	config.StaticUrl = "//static.skullysystems.com"
	config.Mandrill.FromName = "SKULLY"
	config.Mandrill.FromEmail = "noreply@skullysystems.com"

	config.DemoMode = false

	// Only use production credentials if demo mode is off.
	if !config.DemoMode {
		config.Salesforce.ConsumerKey = "3MVG9xOCXq4ID1uElRYWhpUWjXSbiTVg4WO6q9DvWdvBjQ_DFlwSc7jZ9AbY3z9Jv_V29W7xq1nPjTYQhYJqF"
		config.Salesforce.ConsumerSecret = "3811316853831925498"
		config.Salesforce.CallbackURL = "https://admin.crowdstart.io/salesforce/callback"

		config.Stripe.ClientId = "ca_53yyRUNpMtTRUgMlVlLAM3vllY1AVybU"
		config.Stripe.APIKey = "pk_live_APr2mdiUblcOO4c2qTeyQ3hq"
		config.Stripe.APISecret = ""
		config.Stripe.RedirectURL = "https:" + config.UrlFor("platform", "/stripe/callback")
		config.Stripe.WebhookURL = "https:" + config.UrlFor("platform", "/stripe/hook")
	}
	return config
}
