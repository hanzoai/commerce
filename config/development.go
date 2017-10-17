package config

// Development settings
func Development() *Config {
	config := Defaults()

	config.IsDevelopment = true

	config.AutoCompileAssets = false
	config.AutoLoadFixtures = false
	config.DatastoreWarn = true

	config.Protocol = "/"

	config.Prefixes["analytics"] = "/analytics/"
	config.Prefixes["api"] = "/api/"
	config.Prefixes["cdn"] = "/cdn/"
	config.Prefixes["dash"] = "/dash/"
	config.Prefixes["default"] = "/"

	config.Hosts["analytics"] = ""
	config.Hosts["api"] = ""
	config.Hosts["cdn"] = ""
	config.Hosts["dash"] = ""
	config.Hosts["default"] = ""

	config.StaticUrl = "/static"

	config.Ethereum.TestPassword = ""
	config.Ethereum.DepositPassword = ""
	config.Ethereum.MainNetNodes = []string{"35.202.166.74"}
	config.Ethereum.TestNetNodes = []string{"35.192.74.139"}

	config.Stripe.ClientId = config.Stripe.DevelopmentClientId
	config.Stripe.PublishableKey = config.Stripe.TestPublishableKey
	config.Stripe.SecretKey = config.Stripe.TestSecretKey
	config.Stripe.RedirectURL = "http://localhost:8080" + config.UrlFor("api", "/stripe/callback")
	config.Stripe.WebhookURL = "http://localhost:8080" + config.UrlFor("api", "/stripe/webhook")

	config.Facebook.AppId = "484263268389194"
	config.Facebook.AppSecret = "e82c15c92f9679a146a136790baf7d67"
	config.Facebook.GraphVersion = "v2.2"

	config.Google.APIKey = "AIzaSyAOPY7nU-UlNRLvZz9D_j2Qm6SBMUvk83w"
	config.Google.Bucket.ImageUploads = "crowdstart-staging-image-uploads"

	// TODO: Create dev versions somehow
	config.Salesforce.ConsumerKey = "3MVG9xOCXq4ID1uElRYWhpUWjXYxIIlf_W1_MSDefMxTxdgMz5aMsZ7uvZ4n8zHI1wq6UREv2KE31Kes_Bq6D"
	config.Salesforce.ConsumerSecret = "2354282251954184740"
	config.Salesforce.CallbackURL = "http://localhost:8080" + config.UrlFor("dash", "/salesforce/callback")

	return config
}
