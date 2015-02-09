package config

// SKULLY Settings
func Skully() *Config {
	config := Production()

	config.Hosts["default"] = "static.skully.com"
	config.Hosts["api"] = "invalid.skully.com" // Setting platform to API temporarily.
	config.Hosts["checkout"] = "secure.skully.com"
	config.Hosts["platform"] = "api.skully.com"
	config.Hosts["preorder"] = "preorder.skully.com"
	config.Hosts["store"] = "store.skully.com"

	config.CookieDomain = "skully.com"

	config.StaticUrl = "//static.skully.com"
	config.Mandrill.FromName = "SKULLY"
	config.Mandrill.FromEmail = "dev@hanzo.ai"

	config.DemoMode = false

	// Only use production credentials if demo mode is off.
	if !config.DemoMode {
		config.Salesforce.ConsumerKey = "3MVG9xOCXq4ID1uElRYWhpUWjXSbiTVg4WO6q9DvWdvBjQ_DFlwSc7jZ9AbY3z9Jv_V29W7xq1nPjTYQhYJqF"
		config.Salesforce.ConsumerSecret = "3811316853831925498"
		config.Salesforce.CallbackURL = "https://admin.crowdstart.io/salesforce/callback"

		config.Stripe.ClientId = "ca_REDACTED"
		config.Stripe.APIKey = "pk_live_REDACTED"
		config.Stripe.APISecret = ""
		config.Stripe.RedirectURL = "https:" + config.UrlFor("platform", "/stripe/callback")
		config.Stripe.WebhookURL = "https:" + config.UrlFor("platform", "/stripe/hook")
	}

	config.Google.APIKey = "AIza_REDACTED"
	config.Google.Bucket.ImageUploads = "skully-image-uploads"

	return config
}
