package config

// SKULLY Settings
func Skully() *Config {
	config := Production()

	config.Hosts["default"] = "static.skully.com"
	config.Hosts["store"] = "store.skully.com"
	config.Hosts["checkout"] = "secure.skully.com"
	config.Hosts["preorder"] = "preorder.skully.com"

	config.Hosts["api"] = "api-dot-crowdstart-skully.appspot.com"
	config.Hosts["platform"] = "platform-dot-crowdstart-skully.appspot.com"

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

		config.Stripe.ClientId = "ca_53yyRUNpMtTRUgMlVlLAM3vllY1AVybU"
		config.Stripe.APIKey = "pk_live_APr2mdiUblcOO4c2qTeyQ3hq"
		config.Stripe.APISecret = ""
		config.Stripe.RedirectURL = "https:" + config.UrlFor("platform", "/stripe/callback")
		config.Stripe.WebhookURL = "https:" + config.UrlFor("platform", "/stripe/hook")
	}

	config.Google.APIKey = "AIzaSyBakv1fgy0VkQhe_Gv2yK6qdqrzxMKp_WI"
	config.Google.Bucket.ImageUploads = "skully-image-uploads"

	return config
}
