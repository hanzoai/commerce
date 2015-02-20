package config

// Production Settings
func Production() *Config {
	config := Defaults()

	config.SentryDSN = "https://4daf3e86c2744df4b932abbe4eb48aa8:27fa30055d9747e795ca05d5ffb96f0c@app.getsentry.com/32164"

	config.IsProduction = true

	config.Prefixes["default"] = "/"
	config.Prefixes["api"] = "/"
	config.Prefixes["checkout"] = "/"
	config.Prefixes["platform"] = "/"
	config.Prefixes["preorder"] = "/"
	config.Prefixes["store"] = "/"

	config.Hosts["default"] = "static.crowdstart.io"
	config.Hosts["api"] = "api.crowdstart.io"
	config.Hosts["checkout"] = "secure.crowdstart.io"
	config.Hosts["platform"] = "admin.crowdstart.io"
	config.Hosts["preorder"] = "preorder.crowdstart.io"
	config.Hosts["store"] = "store.crowdstart.io"

	config.StaticUrl = "//static.crowdstart.io"

	config.Mandrill.APIKey = "wJ3LGLp5ZOUZlSH8wwqmTg"

	config.Salesforce.CallbackURL = "https:" + config.UrlFor("platform", "/salesforce/callback")
	config.Stripe.RedirectURL = "https:" + config.UrlFor("platform", "/stripe/callback")
	config.Stripe.WebhookURL = "https:" + config.UrlFor("platform", "/stripe/hook")

	config.DemoMode = true

	// Only use production credentials if demo mode is off.
	if !config.DemoMode {
		config.Salesforce.ConsumerKey = "3MVG9xOCXq4ID1uElRYWhpUWjXSbiTVg4WO6q9DvWdvBjQ_DFlwSc7jZ9AbY3z9Jv_V29W7xq1nPjTYQhYJqF"
		config.Salesforce.ConsumerSecret = "3811316853831925498"

		config.Stripe.ClientId = "ca_53yyRUNpMtTRUgMlVlLAM3vllY1AVybU"
		config.Stripe.APIKey = "pk_live_APr2mdiUblcOO4c2qTeyQ3hq"
		config.Stripe.APISecret = ""
	}

	return config
}
