package config

import (
	"appengine"
)

type Config struct {
	AutoCompileAssets bool
	Stripe            struct {
		ClientId    string
		APIKey      string
		APISecret   string
		RedirectURL string
		WebhookURL  string
	}
}

func Defaults() *Config {
	return new(Config)
}

func Development() *Config {
	config := Defaults()
	config.AutoCompileAssets = false
	config.Stripe.ClientId = "ca_53yyPzxlPsdAtzMEIuS2mXYDp4FFXLmm"
	config.Stripe.APIKey = "pk_test_ucSTeAAtkSXVEg713ir40UhX"
	config.Stripe.APISecret = ""
	config.Stripe.RedirectURL = "http://localhost:8080/admin/stripe/callback"
	config.Stripe.WebhookURL = "http://localhost:8080/admin/stripe/hook"
	return config
}

func Production() *Config {
	config := Defaults()
	config.Stripe.ClientId = "ca_53yyRUNpMtTRUgMlVlLAM3vllY1AVybU"
	config.Stripe.APIKey = "pk_live_APr2mdiUblcOO4c2qTeyQ3hq"
	config.Stripe.APISecret = ""
	config.Stripe.RedirectURL = "https://secure.crowdstart.io/admin/stripe/callback"
	config.Stripe.WebhookURL = "https://secure.crowdstart.io/admin/stripe/hook"
	return config
}

func Get() *Config {
	if appengine.IsDevAppServer() {
		return Development()
	} else {
		return Production()
	}
}
