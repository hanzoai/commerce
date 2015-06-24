package config

// SKULLY Settings
func Skully() *Config {
	config := Production()

	config.Hosts["api"] = "api.crowdstart.skully.com"
	config.Hosts["checkout"] = "secure.skully.com"
	config.Hosts["default"] = "static.skully.com"
	config.Hosts["platform"] = "crowdstart.skully.com"
	config.Hosts["preorder"] = "preorder.skully.com"
	config.Hosts["store"] = "store.skully.com"

	config.CookieDomain = "skully.com"

	config.StaticUrl = "//static.skully.com"
	config.Mandrill.FromName = "SKULLY"
	config.Mandrill.FromEmail = "dev@hanzo.ai"

	config.Salesforce.CallbackURL = "https:" + config.UrlFor("platform", "/salesforce/callback")
	config.Stripe.RedirectURL = "https:" + config.UrlFor("platform", "/stripe/callback")
	config.Stripe.WebhookURL = "https:" + config.UrlFor("platform", "/stripe/hook")

	config.Salesforce.ConsumerKey = "3MVG9xOCXq4ID1uElRYWhpUWjXSbiTVg4WO6q9DvWdvBjQ_DFlwSc7jZ9AbY3z9Jv_V29W7xq1nPjTYQhYJqF"
	config.Salesforce.ConsumerSecret = "3811316853831925498"

	config.Stripe.ClientId = "ca_REDACTED"
	config.Stripe.APIKey = "pk_live_REDACTED"
	config.Stripe.APISecret = ""

	config.Google.APIKey = "AIza_REDACTED"
	config.Google.Bucket.ImageUploads = "skully-images"

	config.Discourse.URL = "https://owners.skully.com"
	config.Discourse.Secret = "e836445b10e4085d6b225b61d209edf15"

	return config
}
