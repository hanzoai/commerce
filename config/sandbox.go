package config

// Sandbox Settings
func Sandbox() *Config {
	config := Production()

	config.IsProduction = false
	config.IsStaging = false
	config.IsSandbox = true

	config.Hosts["default"] = "default-dot-crowdstart-sandbox.appspot.com"
	config.Hosts["api"] = "sandbox.crowdstart.com"
	config.Hosts["checkout"] = "checkout-dot-crowdstart-sandbox.appspot.com"
	config.Hosts["platform"] = "platform-dot-crowdstart-sandbox.appspot.com"
	config.Hosts["preorder"] = "preorder-dot-crowdstart-sandbox.appspot.com"
	config.Hosts["store"] = "store-dot-crowdstart-sandbox.appspot.com"

	config.StaticUrl = "//static-dot-crowdstart-sandbox.appspot.com"

	config.Salesforce.CallbackURL = "https:" + config.UrlFor("platform", "/salesforce/callback")
	config.Stripe.RedirectURL = "https:" + config.UrlFor("platform", "/stripe/callback")
	config.Stripe.WebhookURL = "https:" + config.UrlFor("platform", "/stripe/hook")

	config.Salesforce.ConsumerKey = "3MVG9xOCXq4ID1uElRYWhpUWjXYxIIlf_W1_MSDefMxTxdgMz5aMsZ7uvZ4n8zHI1wq6UREv2KE31Kes_Bq6D"
	config.Salesforce.ConsumerSecret = "2354282251954184740"

	config.Stripe.ClientId = "ca_53yyPzxlPsdAtzMEIuS2mXYDp4FFXLmm"
	config.Stripe.APIKey = "pk_test_ucSTeAAtkSXVEg713ir40UhX"
	config.Stripe.APISecret = ""

	config.Mandrill.APIKey = "wJ3LGLp5ZOUZlSH8wwqmTg"

	config.Google.APIKey = "AIzaSyAOPY7nU-UlNRLvZz9D_j2Qm6SBMUvk83w"
	config.Google.Bucket.ImageUploads = "crowdstart-staging-image-uploads"

	return config
}
