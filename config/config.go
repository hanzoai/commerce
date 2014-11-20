package config

import (
	"appengine"
	"strings"
)

const demoMode = true

type Config struct {
	AutoCompileAssets bool
	Prefixes          map[string]string
	Hosts             map[string]string
	Stripe            struct {
		ClientId    string
		APIKey      string
		APISecret   string
		RedirectURL string
		WebhookURL  string
	}
}

func (c Config) URLFor(moduleName, domain string) string {
	// Build URL for module.
	url := c.Hosts[moduleName] + c.Prefixes[moduleName]

	// If module is hosted, return relative to that root domain.
	if domain != "" {
		url = strings.Replace(url, "crowdstart.io", domain, 1)

	}

	return url
}

func Defaults() *Config {
	return new(Config)
}

func Development() *Config {
	config := Defaults()
	config.AutoCompileAssets = false

	config.Prefixes["api"] = "/v1/"
	config.Prefixes["store"] = "/"
	config.Prefixes["checkout"] = "/checkout/"
	config.Prefixes["preorder"] = "/preorder/"

	config.Hosts["api"] = "localhost:8080"
	config.Hosts["store"] = "localhost:8080"
	config.Hosts["checkout"] = "localhost:8080"
	config.Hosts["preorder"] = "localhost:8080"

	config.Stripe.ClientId = "ca_53yyPzxlPsdAtzMEIuS2mXYDp4FFXLmm"
	config.Stripe.APIKey = "pk_test_ucSTeAAtkSXVEg713ir40UhX"
	config.Stripe.APISecret = ""
	config.Stripe.RedirectURL = "http://localhost:8080/admin/stripe/callback"
	config.Stripe.WebhookURL = "http://localhost:8080/admin/stripe/hook"
	return config
}

func Production() *Config {
	config := Defaults()

	config.Prefixes["api"] = "/v1"
	config.Prefixes["store"] = "/"
	config.Prefixes["checkout"] = "/"
	config.Prefixes["preorder"] = "/"

	config.Hosts["api"] = "api.crowdstart.io"
	config.Hosts["store"] = "store.crowdstart.io"
	config.Hosts["checkout"] = "secure.crowdstart.io"
	config.Hosts["preorder"] = "preorder.crowdstart.io"

	config.Stripe.ClientId = "ca_53yyRUNpMtTRUgMlVlLAM3vllY1AVybU"
	config.Stripe.APIKey = "pk_live_APr2mdiUblcOO4c2qTeyQ3hq"
	config.Stripe.APISecret = ""
	config.Stripe.RedirectURL = "https://secure.crowdstart.io/admin/stripe/callback"
	config.Stripe.WebhookURL = "https://secure.crowdstart.io/admin/stripe/hook"
	return config
}

func Get() *Config {
	if demoMode || appengine.IsDevAppServer() {
		return Development()
	} else {
		return Production()
	}
}
