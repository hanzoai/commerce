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
			"ca_REDACTED",
			"pk_test_REDACTED",
			"",
			"http://localhost:8080/stripe/callback",
			"http://localhost:8080/stripe/hook",
		},
	}
}

func Production() *Config {
	return &Config{
		Stripe: Stripe{
			"ca_REDACTED",
			"pk_live_REDACTED",
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
