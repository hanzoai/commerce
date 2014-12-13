package config

// Development settings
func Development() *Config {
	config := Defaults()

	config.IsDevelopment = true

	config.AutoCompileAssets = false
	config.AutoLoadFixtures = true

	config.Prefixes["default"] = "/"
	config.Prefixes["api"] = "/api/"
	config.Prefixes["checkout"] = "/checkout/"
	config.Prefixes["platform"] = "/admin/"
	config.Prefixes["preorder"] = "/preorder/"
	config.Prefixes["store"] = "/store/"

	config.Hosts["default"] = "localhost:8080"
	config.Hosts["api"] = "localhost:8080"
	config.Hosts["checkout"] = "localhost:8080"
	config.Hosts["platform"] = "localhost:8080"
	config.Hosts["preorder"] = "localhost:8080"
	config.Hosts["store"] = "localhost:8080"

	config.StaticUrl = "//localhost:8080/static"

	// TODO: Create dev versions somehow
	config.Salesforce.ConsumerKey = "3MVG9xOCXq4ID1uElRYWhpUWjXSbiTVg4WO6q9DvWdvBjQ_DFlwSc7jZ9AbY3z9Jv_V29W7xq1nPjTYQhYJqF"
	config.Salesforce.ConsumerSecret = "3811316853831925498"
	config.Salesforce.CallbackURL = "https://admin.crowdstart.io/salesforce/callback"

	config.Stripe.ClientId = "ca_53yyPzxlPsdAtzMEIuS2mXYDp4FFXLmm"
	config.Stripe.APIKey = "pk_test_ucSTeAAtkSXVEg713ir40UhX"
	config.Stripe.APISecret = ""
	config.Stripe.RedirectURL = "http:" + config.UrlFor("platform", "/stripe/callback")
	config.Stripe.WebhookURL = "http:" + config.UrlFor("platform", "/stripe/hook")
	return config
}
