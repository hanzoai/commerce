package config

// Sandbox Settings
func Sandbox() *Config {
	config := Production()

	config.IsProduction = false
	config.IsStaging = false
	config.IsSandbox = true

	// Only modules active in sandbox
	config.Hosts["default"] = "sandbox.hanzo.io"
	config.Hosts["analytics"] = "analytics.sandbox.hanzo.io"
	config.Hosts["api"] = "api.sandbox.hanzo.io"

	// Disabled but configured nonetheless
	config.Hosts["checkout"] = "checkout-dot-crowdstart-sandbox.appspot.com"
	config.Hosts["platform"] = "platform-dot-crowdstart-sandbox.appspot.com"
	config.Hosts["preorder"] = "preorder-dot-crowdstart-sandbox.appspot.com"
	config.Hosts["store"] = "store-dot-crowdstart-sandbox.appspot.com"

	config.StaticUrl = "//static-dot-crowdstart-sandbox.appspot.com"

	config.Stripe.ClientId = config.Stripe.DevelopmentClientId
	config.Stripe.PublishableKey = config.Stripe.TestPublishableKey
	config.Stripe.SecretKey = config.Stripe.TestSecretKey
	config.Stripe.RedirectURL = "https:" + config.UrlFor("platform", "/stripe/callback")
	config.Stripe.WebhookURL = "https:" + config.UrlFor("platform", "/stripe/hook")

	config.Google.APIKey = "AIzaSyAOPY7nU-UlNRLvZz9D_j2Qm6SBMUvk83w"
	config.Google.Bucket.ImageUploads = "crowdstart-staging-image-uploads"

	config.Mandrill.APIKey = "wJ3LGLp5ZOUZlSH8wwqmTg"

	config.Salesforce.ConsumerKey = "3MVG9xOCXq4ID1uElRYWhpUWjXYxIIlf_W1_MSDefMxTxdgMz5aMsZ7uvZ4n8zHI1wq6UREv2KE31Kes_Bq6D"
	config.Salesforce.ConsumerSecret = "2354282251954184740"
	config.Salesforce.CallbackURL = "https:" + config.UrlFor("platform", "/salesforce/callback")

	return config
}
