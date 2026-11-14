package stripe

import (
	"strings"

	"github.com/hanzoai/commerce/payment/processor"
)

// Credentials are the deployment's Stripe API keys of one kind (test or live).
type Credentials struct {
	SecretKey      string
	PublishableKey string
}

// Deployment is the deployment's Stripe keys of one kind, from the routing policy's
// stripe accounts: the "test" account for test, the "live" account for live. An account
// holds the secret key (read from KMS) and the publishable key.
//
// A key of the other kind is dropped rather than used. Stripe names a key's kind
// in its prefix (sk_test_/rk_test_/pk_test_ versus sk_live_/rk_live_/pk_live_),
// so a test org can never charge the live account, and a live org never takes
// test money it would credit as real.
func Deployment(test bool) Credentials {
	if test {
		a := processor.AccountOf(processor.Stripe, "test")
		return Credentials{
			SecretKey:      kind(strings.TrimSpace(a.Secret("secretKey")), "_test_", "sk", "rk"),
			PublishableKey: kind(strings.TrimSpace(a.Setting("publishableKey")), "_test_", "pk"),
		}
	}
	a := processor.AccountOf(processor.Stripe, "live")
	return Credentials{
		SecretKey:      kind(strings.TrimSpace(a.Secret("secretKey")), "_live_", "sk", "rk"),
		PublishableKey: kind(strings.TrimSpace(a.Setting("publishableKey")), "_live_", "pk"),
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
