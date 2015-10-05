package config

import "crowdstart.com/util/log"

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

	config.Hosts["analytics"] = "analytics.staging.crowdstart.com"
	config.Hosts["api"] = "api.staging.crowdstart.com"
	config.Hosts["cdn"] = "cdn.staging.crowdstart.com"
	config.Hosts["checkout"] = "checkout.staging.crowdstart.com"
	config.Hosts["default"] = "default.staging.crowdstart.com"
	config.Hosts["platform"] = "platform.staging.crowdstart.com"
	config.Hosts["preorder"] = "preorder.staging.crowdstart.com"
	config.Hosts["store"] = "store.staging.crowdstart.com"

	config.StaticUrl = "//static.staging.crowdstart.com"

	config.Paypal.ApplicationId = "APP-80W284485P519543T"
	config.Paypal.SecurityUserId = "paypal_api1.verus.io"
	config.Paypal.SecurityPassword = "EH4HZWXCWXVDYWM2"
	config.Paypal.SecuritySignature = "AJd-SFo6hKDOAw2o1mufYejLBcKvAMX-QHZ9..uLkFX45mnUulajOXBJ"
	config.Paypal.Api = "https://svcs.sandbox.paypal.com"

	config.Stripe.ClientId = config.Stripe.DevelopmentClientId
	config.Stripe.PublishableKey = config.Stripe.TestPublishableKey
	config.Stripe.SecretKey = config.Stripe.TestSecretKey
	config.Stripe.RedirectURL = "https:" + config.UrlFor("platform", "/stripe/callback")
	config.Stripe.WebhookURL = "https:" + config.UrlFor("platform", "/stripe/hook")

	config.Facebook.AppId = "484263268389194"
	config.Facebook.AppSecret = "e82c15c92f9679a146a136790baf7d67"
	config.Facebook.GraphVersion = "v2.2"

	config.Mandrill.APIKey = "wJ3LGLp5ZOUZlSH8wwqmTg"

	config.Google.APIKey = "AIzaSyAOPY7nU-UlNRLvZz9D_j2Qm6SBMUvk83w"
	config.Google.Bucket.ImageUploads = "crowdstart-staging-image-uploads"

	config.Salesforce.ConsumerKey = "3MVG9xOCXq4ID1uElRYWhpUWjXYxIIlf_W1_MSDefMxTxdgMz5aMsZ7uvZ4n8zHI1wq6UREv2KE31Kes_Bq6D"
	config.Salesforce.ConsumerSecret = "2354282251954184740"
	config.Salesforce.CallbackURL = "https:" + config.UrlFor("platform", "/salesforce/callback")

	log.SetVerbose(true) // Set verbose logging in staging

	return config
}
