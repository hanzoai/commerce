package stripe

import (
	"os"
	"strings"
)

// Credentials are the deployment's Stripe API keys of one kind (test or live).
type Credentials struct {
	SecretKey      string
	PublishableKey string
}

// FromEnv reads the deployment's Stripe keys of one kind: STRIPE_TEST_SECRET_KEY
// and STRIPE_TEST_PUBLISHABLE_KEY for test, STRIPE_SECRET_KEY and
// STRIPE_PUBLISHABLE_KEY for live.
//
// A key of the other kind is dropped rather than used. Stripe names a key's kind
// in its prefix (sk_test_/rk_test_/pk_test_ versus sk_live_/rk_live_/pk_live_),
// so a test org can never charge the live account, and a live org never takes
// test money it would credit as real.
func FromEnv(test bool) Credentials {
	if test {
		return Credentials{
			SecretKey:      kind(envValue("STRIPE_TEST_SECRET_KEY"), "_test_", "sk", "rk"),
			PublishableKey: kind(envValue("STRIPE_TEST_PUBLISHABLE_KEY"), "_test_", "pk"),
		}
	}
	return Credentials{
		SecretKey:      kind(envValue("STRIPE_SECRET_KEY"), "_live_", "sk", "rk"),
		PublishableKey: kind(envValue("STRIPE_PUBLISHABLE_KEY"), "_live_", "pk"),
	}
}

// kind returns key when its prefix is one of prefixes followed by the wanted
// kind ("_test_" or "_live_"), else "". A publishable slot takes only pk_, so a
// secret key misfiled there is never published.
func kind(key, want string, prefixes ...string) string {
	for _, p := range prefixes {
		if strings.HasPrefix(key, p+want) {
			return key
		}
	}
	return ""
}

func envValue(name string) string { return strings.TrimSpace(os.Getenv(name)) }
