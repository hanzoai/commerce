package config

// Production Settings
func Production() *Config {
	config := Defaults()

	config.SentryDSN = "https://4daf3e86c2744df4b932abbe4eb48aa8:27fa30055d9747e795ca05d5ffb96f0c@app.getsentry.com/32164"

	config.IsProduction = true

	config.Prefixes["default"] = "/"
	config.Prefixes["api"] = "/"
	config.Prefixes["checkout"] = "/"
	config.Prefixes["platform"] = "/"
	config.Prefixes["preorder"] = "/"
	config.Prefixes["store"] = "/"

	config.Hosts["default"] = "static.crowdstart.com"
	config.Hosts["api"] = "api.crowdstart.com"
	config.Hosts["checkout"] = "secure.crowdstart.com"
	config.Hosts["platform"] = "www.crowdstart.com"
	config.Hosts["preorder"] = "preorder.crowdstart.com"
	config.Hosts["store"] = "store.crowdstart.com"

	config.StaticUrl = "//static.crowdstart.com"

	config.Mandrill.APIKey = "wJ3LGLp5ZOUZlSH8wwqmTg"

	config.Salesforce.CallbackURL = "https:" + config.UrlFor("platform", "/salesforce/callback")
	config.Stripe.RedirectURL = "https:" + config.UrlFor("platform", "/stripe/callback")
	config.Stripe.WebhookURL = "https:" + config.UrlFor("platform", "/stripe/hook")

	config.Facebook.AppId = "484263268389194"
	config.Facebook.AppSecret = "e82c15c92f9679a146a136790baf7d67"
	config.Facebook.GraphVersion = "v2.2"

	config.DemoMode = false

	config.Salesforce.ConsumerKey = ""
	config.Salesforce.ConsumerSecret = ""

	config.Stripe.ClientId = "ca_53yyRUNpMtTRUgMlVlLAM3vllY1AVybU"
	config.Stripe.APIKey = "pk_live_APr2mdiUblcOO4c2qTeyQ3hq"
	config.Stripe.APISecret = ""

	return config
}
