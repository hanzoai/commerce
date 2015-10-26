package config

// Production Settings
func Production() *Config {
	config := Defaults()

	config.IsProduction = true

	config.Prefixes["analytics"] = "/"
	config.Prefixes["api"] = "/"
	config.Prefixes["cdn"] = "/"
	config.Prefixes["checkout"] = "/"
	config.Prefixes["default"] = "/"
	config.Prefixes["platform"] = "/"
	config.Prefixes["preorder"] = "/"
	config.Prefixes["store"] = "/"

	config.Hosts["analytics"] = "analytics.crowdstart.com"
	config.Hosts["api"] = "api.crowdstart.com"
	config.Hosts["cdn"] = "cdn.crowdstart.com"
	config.Hosts["checkout"] = "secure.crowdstart.com"
	config.Hosts["default"] = "static.crowdstart.com"
	config.Hosts["platform"] = "www.crowdstart.com"
	config.Hosts["preorder"] = "preorder.crowdstart.com"
	config.Hosts["store"] = "store.crowdstart.com"

	config.StaticUrl = "//static.crowdstart.com"

	config.DemoMode = false

	config.Paypal.ApplicationId = ""
	config.Paypal.SecurityUserId = "paypal_api1.verus.io"
	config.Paypal.SecurityPassword = "EH4HZWXCWXVDYWM2"
	config.Paypal.SecuritySignature = "AJd-SFo6hKDOAw2o1mufYejLBcKvAMX-QHZ9..uLkFX45mnUulajOXBJ"
	config.Paypal.Api = ""
	config.Paypal.IpnUrl = "https://www.paypal.com/cgi-bin/webscr"

	config.Stripe.ClientId = config.Stripe.ProductionClientId
	config.Stripe.SecretKey = config.Stripe.LiveSecretKey
	config.Stripe.PublishableKey = config.Stripe.LivePublishablKey
	config.Stripe.RedirectURL = "https:" + config.UrlFor("platform", "/stripe/callback")
	config.Stripe.WebhookURL = "https:" + config.UrlFor("platform", "/stripe/hook")

	config.Facebook.AppId = "484263268389194"
	config.Facebook.AppSecret = "e82c15c92f9679a146a136790baf7d67"
	config.Facebook.GraphVersion = "v2.2"

	config.Mandrill.APIKey = "wJ3LGLp5ZOUZlSH8wwqmTg"

	config.Salesforce.ConsumerKey = ""
	config.Salesforce.ConsumerSecret = ""
	config.Salesforce.CallbackURL = "https:" + config.UrlFor("platform", "/salesforce/callback")

	return config
}
