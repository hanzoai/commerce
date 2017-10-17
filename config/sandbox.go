package config

// Sandbox Settings
func Sandbox() *Config {
	config := Production()

	config.IsProduction = false
	config.IsStaging = false
	config.IsSandbox = true

	// Only modules active in sandbox
	config.Hosts["default"] = "sandbox.hanzo.io"
	config.Hosts["api"] = "api.sandbox.hanzo.io"
	config.Hosts["dash"] = "dash.sandbox.hanzo.io"

	config.Ethereum.TestPassword = ""
	config.Ethereum.DepositPassword = ""
	config.Ethereum.MainNetNodes = []string{"35.202.166.74"}
	config.Ethereum.TestNetNodes = []string{"35.192.74.139"}

	config.StaticUrl = "//static.sandbox.hanzo.io"

	config.Stripe.ClientId = config.Stripe.DevelopmentClientId
	config.Stripe.PublishableKey = config.Stripe.TestPublishableKey
	config.Stripe.SecretKey = config.Stripe.TestSecretKey
	config.Stripe.RedirectURL = "https:" + config.UrlFor("api", "/stripe/callback")
	config.Stripe.WebhookURL = "https:" + config.UrlFor("api", "/stripe/webhook")

	config.Google.APIKey = "AIzaSyAOPY7nU-UlNRLvZz9D_j2Qm6SBMUvk83w"
	config.Google.Bucket.ImageUploads = "hanzo-sandbox-image-uploads"

	config.Mandrill.APIKey = "wJ3LGLp5ZOUZlSH8wwqmTg"

	config.Salesforce.ConsumerKey = "3MVG9xOCXq4ID1uElRYWhpUWjXYxIIlf_W1_MSDefMxTxdgMz5aMsZ7uvZ4n8zHI1wq6UREv2KE31Kes_Bq6D"
	config.Salesforce.ConsumerSecret = "2354282251954184740"
	config.Salesforce.CallbackURL = "https:" + config.UrlFor("dash", "/salesforce/callback")

	return config
}
