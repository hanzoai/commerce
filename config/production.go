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
	config.Hosts["platform"] = "dash.hanzo.io"
	config.Hosts["preorder"] = "preorder.crowdstart.com"
	config.Hosts["store"] = "store.crowdstart.com"

	config.StaticUrl = "//static.crowdstart.com"

	config.DemoMode = false

	config.Paypal.Email = "dev@hanzo.ai"
	config.Paypal.Api = "https://svcs.paypal.com"
	config.Paypal.IpnUrl = "https://api.crowdstart.com/paypal/ipn/"
	config.Paypal.PaypalIpnUrl = "https://www.paypal.com/cgi-bin/webscr"

	config.Stripe.ClientId = config.Stripe.ProductionClientId
	config.Stripe.SecretKey = config.Stripe.LiveSecretKey
	config.Stripe.PublishableKey = config.Stripe.LivePublishablKey
	config.Stripe.RedirectURL = "https:" + config.UrlFor("api", "/stripe/callback")
	config.Stripe.WebhookURL = "https:" + config.UrlFor("api", "/stripe/webhook")

	config.Facebook.AppId = "484263268389194"
	config.Facebook.AppSecret = "e82c15c92f9679a146a136790baf7d67"
	config.Facebook.GraphVersion = "v2.2"

	config.Mandrill.APIKey = "wJ3LGLp5ZOUZlSH8wwqmTg"

	config.Salesforce.ConsumerKey = ""
	config.Salesforce.ConsumerSecret = ""
	config.Salesforce.CallbackURL = "https:" + config.UrlFor("platform", "/salesforce/callback")
	config.Netlify.AccessToken = "1739f774d10d95de710c35a3184c7e71d086e5e750cc99c6648274240e9377de"

	return config
}
