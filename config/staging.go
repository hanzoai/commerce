package config

import "hanzo.io/util/log"

// Staging Settings
func Staging() *Config {
	config := Production()

	config.IsProduction = false
	config.IsStaging = true

	config.Prefixes["analytics"] = "/"
	config.Prefixes["api"] = "/"
	config.Prefixes["cdn"] = "/"
	config.Prefixes["checkout"] = "/"
	config.Prefixes["default"] = "/"
	config.Prefixes["platform"] = "/"
	config.Prefixes["preorder"] = "/"
	config.Prefixes["store"] = "/"

	config.Hosts["analytics"] = "analytics.staging.hanzo.io"
	config.Hosts["api"] = "api.staging.hanzo.io"
	config.Hosts["cdn"] = "cdn.staging.hanzo.io"
	config.Hosts["checkout"] = "checkout.staging.hanzo.io"
	config.Hosts["default"] = "default.staging.hanzo.io"
	config.Hosts["platform"] = "platform.staging.hanzo.io"
	config.Hosts["preorder"] = "preorder.staging.hanzo.io"
	config.Hosts["store"] = "store.staging.hanzo.io"

	config.StaticUrl = "//static.staging.hanzo.io"

	config.Paypal.Email = "dev@hanzo.ai"
	config.Paypal.Api = "https://svcs.sandbox.paypal.com"
	config.Paypal.IpnUrl = "https://api.staging.hanzo.io/paypal/ipn/"
	config.Paypal.PaypalIpnUrl = "https://www.sandbox.paypal.com/cgi-bin/webscr"

	config.Stripe.ClientId = config.Stripe.DevelopmentClientId
	config.Stripe.PublishableKey = config.Stripe.TestPublishableKey
	config.Stripe.SecretKey = config.Stripe.TestSecretKey
	config.Stripe.RedirectURL = "https:" + config.UrlFor("api", "/stripe/callback")
	config.Stripe.WebhookURL = "https:" + config.UrlFor("api", "/stripe/webhook")

	config.Facebook.AppId = "484263268389194"
	config.Facebook.AppSecret = "e82c15c92f9679a146a136790baf7d67"
	config.Facebook.GraphVersion = "v2.2"

	config.Mandrill.APIKey = "wJ3LGLp5ZOUZlSH8wwqmTg"

	config.Google.APIKey = "AIzaSyAOPY7nU-UlNRLvZz9D_j2Qm6SBMUvk83w"
	config.Google.Bucket.ImageUploads = "crowdstart-staging-image-uploads"

	config.Salesforce.ConsumerKey = "3MVG9xOCXq4ID1uElRYWhpUWjXYxIIlf_W1_MSDefMxTxdgMz5aMsZ7uvZ4n8zHI1wq6UREv2KE31Kes_Bq6D"
	config.Salesforce.ConsumerSecret = "2354282251954184740"
	config.Salesforce.CallbackURL = "https:" + config.UrlFor("platform", "/salesforce/callback")

	config.Netlify.AccessToken = "cb55596d4400897691b51df746c9007ea0f073139d1ec0af705b0a3c77d70621"

	log.SetVerbose(true) // Set verbose logging in staging

	return config
}
