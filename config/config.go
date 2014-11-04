package config

import (
	"appengine"
)

type Stripe struct {
	ClientId    string
	APIKey      string
	APISecret   string
	RedirectURI string
	RedirectURL string
}

type Config struct {
	Stripe Stripe
}

func Development() *Config {
	return &Config{
		Stripe: Stripe{
			"ca_53yyPzxlPsdAtzMEIuS2mXYDp4FFXLmm",
			"pk_test_ucSTeAAtkSXVEg713ir40UhX",
			"",
			"http://localhost:8080/stripe/callback",
			"http://localhost:8080/stripe/hook",
		},
	}
}

func Production() *Config {
	return &Config{
		Stripe: Stripe{
			"ca_53yyRUNpMtTRUgMlVlLAM3vllY1AVybU",
			"pk_live_APr2mdiUblcOO4c2qTeyQ3hq",
			"",
			"https://secure.crowdstart.io/stripe/callback",
			"https://secure.crowdstart.io/stripe/hook",
		},
	}
}

func Get() *Config {
	if appengine.IsDevAppServer() {
		return Development()
	} else {
		return Production()
	}
}
