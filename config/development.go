package config

// Development settings
func Development() *Config {
	config := Defaults()

	config.IsDevelopment = true

	config.AutoCompileAssets = false
	config.AutoLoadFixtures = true
	config.DatastoreWarn = true

	config.Protocol = "//localhost:8080/" // Localhost

	config.Prefixes["default"] = "/"
	config.Prefixes["api"] = "/api/"
	config.Prefixes["checkout"] = "/checkout/"
	config.Prefixes["platform"] = "/platform/"
	config.Prefixes["preorder"] = "/preorder/"
	config.Prefixes["store"] = "/store/"

	config.Hosts["default"] = ""
	config.Hosts["api"] = ""
	config.Hosts["checkout"] = ""
	config.Hosts["platform"] = ""
	config.Hosts["preorder"] = ""
	config.Hosts["store"] = ""

	config.StaticUrl = "/static"

	// TODO: Create dev versions somehow
	config.Salesforce.ConsumerKey = "3MVG9xOCXq4ID1uElRYWhpUWjXYxIIlf_W1_MSDefMxTxdgMz5aMsZ7uvZ4n8zHI1wq6UREv2KE31Kes_Bq6D"
	config.Salesforce.ConsumerSecret = "2354282251954184740"
	config.Salesforce.CallbackURL = "https:" + config.UrlFor("platform", "/salesforce/callback")

	config.Stripe.ClientId = "ca_53yyPzxlPsdAtzMEIuS2mXYDp4FFXLmm"
	config.Stripe.APIKey = "pk_test_ucSTeAAtkSXVEg713ir40UhX"
	config.Stripe.APISecret = ""
	config.Stripe.RedirectURL = "http:" + config.UrlFor("platform", "/stripe/callback")
	config.Stripe.WebhookURL = "http:" + config.UrlFor("platform", "/stripe/hook")

	config.Google.APIKey = "AIzaSyAOPY7nU-UlNRLvZz9D_j2Qm6SBMUvk83w"
	config.Google.Bucket.ImageUploads = "crowdstart-staging-image-uploads"

	return config
}
