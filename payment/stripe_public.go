package payment

import (
	"strings"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment/providers/stripe"
)

// StripePublic is the PUBLIC Stripe.js configuration a browser needs to mount
// Stripe's card form for an org: the publishable key and the kind of money it
// moves. Stripe publishes the publishable key to every page that loads
// Stripe.js; the secret key never leaves the server.
type StripePublic struct {
	PublishableKey string `json:"publishableKey"`
	Environment    string `json:"environment"` // "test" | "live"
	Live           bool   `json:"live"`
}

// StripeKeys resolves the Stripe keys an org charges with, through the single
// test-mode authority (org.TestMode): the org's own (Hydrate fills them from
// KMS), else the deployment's of the same kind (stripe.FromEnv). The secret and
// the publishable key always come from ONE account, so the card the browser
// tokenizes is a card the server can charge.
func StripeKeys(org *organization.Organization) stripe.Credentials {
	if org == nil {
		org = &organization.Organization{}
	}
	test := org.TestMode()
	own := org.Stripe.Live
	if test {
		own = org.Stripe.Test
	}
	if sk := strings.TrimSpace(own.AccessToken); sk != "" {
		// The org's own keys are held to the same rule as the deployment's: a secret
		// key of the org's mode (sk_/rk_ + _test_ or _live_), and a publishable key
		// (pk_) of the same mode. A key of the other mode, or of another kind, is no
		// key — the org has none, rather than one that charges the wrong money.
		want := "_live_"
		if test {
			want = "_test_"
		}
		if kind(sk, want, "sk", "rk") == "" {
			return stripe.Credentials{}
		}
		return stripe.Credentials{SecretKey: sk, PublishableKey: kind(strings.TrimSpace(own.PublishableKey), want, "pk")}
	}
	return stripe.Deployment(test)
}

// StripePublicConfig is the public half of StripeKeys. An org whose account has
// no publishable key has no card form to mount, and says so with an empty key.
func StripePublicConfig(org *organization.Organization) StripePublic {
	if org == nil {
		org = &organization.Organization{}
	}
	keys := StripeKeys(org)
	pk := keys.PublishableKey
	if keys.SecretKey == "" {
		pk = ""
	}
	return StripePublic{
		PublishableKey: pk,
		Environment:    org.StripeEnvironment(),
		Live:           !org.TestMode(),
	}
}

// StripeOwned reports whether the org holds Stripe keys of its own, the same
// test StripeKeys makes before it falls back to the deployment's.
func StripeOwned(org *organization.Organization) bool {
	if org == nil {
		return false
	}
	own := org.Stripe.Live
	if org.TestMode() {
		own = org.Stripe.Test
	}
	return strings.TrimSpace(own.AccessToken) != ""
}

// kind returns key when it is one of prefixes followed by the wanted mode.
func kind(key, want string, prefixes ...string) string {
	for _, p := range prefixes {
		if strings.HasPrefix(key, p+want) {
			return key
		}
	}
	return ""
}

// OwnStripeSecret is the org's OWN secret key for its mode, checked as StripeKeys
// checks it, and never the deployment's: a merchant's hosted checkout charges the
// merchant's account only. "" when it holds none, or one of the wrong mode.
func OwnStripeSecret(org *organization.Organization) string {
	if org == nil {
		return ""
	}
	own, want := org.Stripe.Live, "_live_"
	if org.TestMode() {
		own, want = org.Stripe.Test, "_test_"
	}
	return kind(strings.TrimSpace(own.AccessToken), want, "sk", "rk")
}
