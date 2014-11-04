package config

import (
	"appengine"
)

type Config struct {
	Stripe struct {
		ClientId     string
		ClientSecret string
		APIKey       string
		RedirectURI  string
		RedirectURL  string
	}
}

func Development() Config {
	return &Config{
		Stripe: Stripe{
			"ca_53yyPzxlPsdAtzMEIuS2mXYDp4FFXLmm",
			"",
			"pk_test_ucSTeAAtkSXVEg713ir40UhX"
			"http://localhost:8080/stripe/redirect",
			"http://localhost:8080/stripe/hook",
		},
	}
}

func Production() Config {
	return &Config{
		Stripe: Stripe{
			"ca_53yyRUNpMtTRUgMlVlLAM3vllY1AVybU",
			"",
			"pk_live_APr2mdiUblcOO4c2qTeyQ3hq",
			"https://secure.crowdstart.io/stripe/redirect",
			"https://secure.crowdstart.io/stripe/hook",
		},
	}
}

func Get() Config {
	if appengine.IsDevAppServer() {
		return Development()
	} else {
		return Production()
	}
}
