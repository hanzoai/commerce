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
	// Parity
	// config.Ethereum.MainNetNodes = []string{"http://35.192.92.62:13264"}
	// Geth
	config.Ethereum.MainNetNodes = []string{"http://35.193.184.247:13264"}
	config.Ethereum.TestNetNodes = []string{"http://35.192.74.139:13264"}
	config.Ethereum.WebhookPassword = ""

	config.Bitcoin.TestPassword = ""
	config.Bitcoin.DepositPassword = ""
	config.Bitcoin.MainNetNodes = []string{"http://35.192.49.112:19283"}
	config.Bitcoin.MainNetUsernames = []string{""}
	config.Bitcoin.MainNetPasswords = []string{""}
	config.Bitcoin.TestNetNodes = []string{"http://104.154.51.133:19283"}
	config.Bitcoin.TestNetUsernames = []string{""}
	config.Bitcoin.TestNetPasswords = []string{""}
	config.Bitcoin.WebhookPassword = ""

	config.Stripe.ClientId = config.Stripe.DevelopmentClientId
	config.Stripe.PublishableKey = config.Stripe.TestPublishableKey
	config.Stripe.SecretKey = config.Stripe.TestSecretKey
	config.Stripe.RedirectURL = "http://localhost:8080" + config.UrlFor("platform", "/stripe/callback")
	config.Stripe.WebhookURL = "http://localhost:8080" + config.UrlFor("platform", "/stripe/hook")

	config.Facebook.AppId = "484263268389194"
	config.Facebook.AppSecret = "e82c15c92f9679a146a136790baf7d67"
	config.Facebook.GraphVersion = "v2.2"

	config.Google.APIKey = "AIza_REDACTED"
	config.Google.Bucket.ImageUploads = "hanzo-staging-image-uploads"

	// TODO: Create dev versions somehow
	config.Salesforce.ConsumerKey = "3MVG9xOCXq4ID1uElRYWhpUWjXYxIIlf_W1_MSDefMxTxdgMz5aMsZ7uvZ4n8zHI1wq6UREv2KE31Kes_Bq6D"
	config.Salesforce.ConsumerSecret = "2354282251954184740"
	config.Salesforce.CallbackURL = "http://localhost:8080" + config.UrlFor("dash", "/salesforce/callback")

	config.Cloudflare.Email = "dev@hanzo.ai"
	config.Cloudflare.Key = ""
	config.Cloudflare.ZoneId = "de1ce33d1ff8b42e40d8984cd915b95a"

	config.Netlify.AccessToken = "6e14ab7d48eaefcca030f86124aee0a937c31ce3030db7699dba5473d9c8c0b9"

	return config
}
