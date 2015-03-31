package config

// SKULLY Settings
func Skully() *Config {
	config := Production()

	config.Hosts["default"] = "static.skully.com"
	config.Hosts["checkout"] = "secure.skully.com"
	config.Hosts["platform"] = "crowdstart.skully.com"
	config.Hosts["preorder"] = "preorder.skully.com"
	config.Hosts["store"] = "store.skully.com"

	config.Hosts["api"] = "api-dot-crowdstart-skully.appspot.com"

	config.CookieDomain = "skully.com"

	config.StaticUrl = "//static.skully.com"

	config.DemoMode = false

	config.Stripe.ClientId = config.Stripe.ProductionClientId
	config.Stripe.SecretKey = config.Stripe.LiveSecretKey
	config.Stripe.PublishableKey = config.Stripe.LivePublishablKey
	config.Stripe.RedirectURL = "https:" + config.UrlFor("platform", "/stripe/callback")
	config.Stripe.WebhookURL = "https:" + config.UrlFor("platform", "/stripe/hook")

	config.Google.APIKey = "AIzaSyDh2Dnv_pRKdpMi4QUrcxraG7XeniH4JTw"
	config.Google.Bucket.ImageUploads = "skully-images"

	config.Mandrill.FromName = "SKULLY"
	config.Mandrill.FromEmail = "dev@hanzo.ai"

	config.Salesforce.ConsumerKey = "3MVG9xOCXq4ID1uElRYWhpUWjXSbiTVg4WO6q9DvWdvBjQ_DFlwSc7jZ9AbY3z9Jv_V29W7xq1nPjTYQhYJqF"
	config.Salesforce.ConsumerSecret = "3811316853831925498"
	config.Salesforce.CallbackURL = "https:" + config.UrlFor("platform", "/salesforce/callback")

	return config
}
