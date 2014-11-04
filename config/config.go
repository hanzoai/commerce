package config

import (
	"appengine"
)

type Config struct {
	Stripe struct {
		ClientId    string
		RedirectURI string
		RedirectURL string
	}
}

func Development() Config {
	return &Config{
		Stripe: Stripe{
			"ca_REDACTED",
			"http://localhost:8080/stripe/redirect",
			"http://localhost:8080/stripe/hook",
		},
	}
}

func Production() Config {
	return &Config{
		Stripe: Stripe{
			"ca_REDACTED",
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
