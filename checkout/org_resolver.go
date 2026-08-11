// Package checkout — OrgResolver is the canonical org resolver: the IAM org
// IS the org. There is no separate commerce-org registry to seed or drift.
//
// Resolution is host → brand → IAM org slug → Organization → public Org:
//   - brandForHost maps a request host to its brand + IAM app (pay.hanzo.ai →
//     hanzo, pay.lux.network → lux, …). Unknown hosts fall back to the
//     deployment's default org (COMMERCE_DEFAULT_ORG, default "hanzo").
//   - the org's public Square config (application id + location + environment)
//     is resolved by the SAME authority as the charge path
//     (payment.SquarePublicConfig) and projected into the org JSON, so the
//     pay SPA's card iframe initializes with the exact application commerce
//     will charge — no build-time VITE_* env, no per-host seed row.
//
// Resolution never 404s for a well-formed host: a missing org row degrades to
// the brand defaults + env Square config, so "add credits" always renders.
package checkout

import (
	"os"
	"strings"
	"sync"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment"
	"github.com/hanzoai/commerce/payment/processor"
)

// OrgLoader loads an Organization by its IAM slug (read-only). It is injected
// by the binary so this package stays decoupled from datastore wiring; a nil
// loader (or a miss) degrades to a brand-default synthetic org whose Square
// config resolves from the deployment's per-brand env (the cloud-org path).
//
// WARNING: Resolve runs on /v1/commerce/org — a PUBLIC, unauthenticated
// endpoint the pay SPA hits on every boot. Any real loader wired here MUST be
// cached AND deadline-bounded. A naive per-request DB query under an unbounded
// context blocks and can exhaust the connection pool (regression at 1.42.44).
// nil is the correct default: pure host→brand→env, no I/O, no hang.
type OrgLoader func(slug string) (*organization.Organization, bool)

// OrgResolver implements Resolver by mapping the Host header to an IAM org and
// projecting that org into a public Org.
type OrgResolver struct {
	load OrgLoader
}

// NewOrgResolver returns a Resolver backed by the IAM org model. load may be
// nil (every host resolves to a brand-default synthetic org + env Square).
func NewOrgResolver(load OrgLoader) *OrgResolver {
	return &OrgResolver{load: load}
}

// Resolve maps host → brand → org and projects a public Org. It returns
// ErrUnknownOrg only for a malformed Host (empty / control bytes); every
// well-formed host resolves (known brand or the deployment default).
func (r *OrgResolver) Resolve(host string) (Org, error) {
	h := normalizeHost(host)
	if h == "" {
		return Org{}, ErrUnknownOrg
	}
	b := brandForHost(h)

	var org *organization.Organization
	if r.load != nil {
		if o, ok := r.load(b.slug); ok && o != nil {
			org = o
		}
	}
	if org == nil {
		// No org row yet — the brand's env-backed Square config still resolves.
		org = &organization.Organization{}
		org.Name = b.slug
	}

	sq := payment.SquarePublicConfig(org)

	return Org{
		Name: b.slug,
		Brand: Brand{
			DisplayName:  b.displayName,
			LogoURL:      b.logoURL,
			PrimaryColor: b.primaryColor,
		},
		IAM: IAMConfig{
			Issuer:   b.iamIssuer,
			ClientID: b.iamClientID,
		},
		Providers:          enabledProviders(),
		ReturnURLAllowlist: returnHostsFor(b.slug),
		Square: SquarePublic{
			ApplicationID: sq.ApplicationID,
			LocationID:    sq.LocationID,
			Environment:   sq.Environment,
		},
	}, nil
}

// returnHostsFor is the per-brand allowlist of first-party app hosts the pay
// SPA may bounce ?return= back to after checkout/onboarding. The pay matcher
// (isAllowedReturnUrl) requires an EXACT https hostname match, so these are
// full origins, not wildcards — every entry is a brand-controlled app host.
// Bounding ?return= to them keeps open-redirect phishing off the table while
// letting a user return to the app that sent them here (e.g. the playground
// flow: playground.hanzo.bot → pay.hanzo.ai/onboard → back to playground).
//
// This is intentionally a static first-party list, not org-model data: the
// resolver runs nil-loader on the PUBLIC /v1/commerce/org path (no
// per-request DB read — see the OrgLoader warning above), and these hosts are
// deploy-stable. Add a host here when a new first-party app must round-trip
// through pay.
func returnHostsFor(slug string) []string {
	switch slug {
	case "hanzo":
		return []string{
			"https://playground.hanzo.bot",
			"https://hanzo.bot",
			"https://console.hanzo.ai",
			"https://cloud.hanzo.ai",
			// hanzo.app — the builder. Its /billing and /pricing pages send the
			// customer to pay.hanzo.ai for a card top-up or a plan and expect to
			// get them back on /billing afterwards. Without this entry that
			// returnUrl fails the allowlist and the buyer is stranded on pay.
			"https://hanzo.app",
			"https://chat.hanzo.ai",
			"https://hanzo.chat",
			"https://analytics.hanzo.ai",
			"https://platform.hanzo.ai",
			"https://bot.hanzo.ai",
			"https://billing.hanzo.ai",
			"https://s3.hanzo.ai",
			"https://hanzo.ai",
			"https://www.hanzo.ai",
			// hanzo.agency — the agency marketing/onboarding site. Its
			// onboarding checkout mints a real Square link via the authed
			// storefront BFF and bounces the buyer to
			// https://hanzo.agency/onboarding-success on success. A distinct
			// registrable domain, so it belongs in the brand's first-party
			// host list (returnHostsFor), not brandDomains.
			"https://hanzo.agency",
			"https://www.hanzo.agency",
		}
	case "lux":
		return []string{
			"https://lux.network",
			"https://lux.id",
			"https://lux.finance",
			"https://console.lux.network",
		}
	case "zoo":
		return []string{
			"https://zoo.ngo",
			"https://zoo.network",
			"https://zoo.id",
		}
	case "pars":
		return []string{
			"https://pars.network",
			"https://pars.id",
			"https://pars.vote",
		}
	default:
		return nil
	}
}

// enabledProviders is the payment-method surface the pay SPA renders, honoring
// the deploy-wide disabled policy. Square is the fiat card processor (gated on
// the Stripe-off policy so "Square-only fiat" stays consistent with the charge
// path); crypto (MPC) and bank wire are always available. Stripe is NEVER
// surfaced for new charges.
func enabledProviders() []Provider {
	out := make([]Provider, 0, 3)
	if !processor.DisabledByPolicy(processor.Square) {
		out = append(out, Provider{Name: "square", Enabled: true})
	}
	out = append(out, Provider{Name: "crypto", Enabled: true})
	out = append(out, Provider{Name: "wire", Enabled: true})
	return out
}

// ─── Brand map (ONE place) ───────────────────────────────────────────────────

// brand is the per-brand org defaults: the IAM org slug, display branding,
// and the IAM app the SPA signs in against. These mirror the commerce
// deployment's IAM_ISSUER / IAM_ACCEPTED_ISSUERS / IAM_ACCEPTED_AUDIENCES.
type brand struct {
	slug         string
	displayName  string
	logoURL      string
	primaryColor string
	iamIssuer    string
	iamClientID  string
}

// brandDomains maps a registrable brand domain to its brand. Matching is
// exact-suffix (host == domain OR host endsWith "."+domain) so a spoof host
// like "pay.hanzo.ai.evil.com" can never match "hanzo.ai".
type domainBrand struct {
	domain string
	brand  brand
}

var brandDomains = []domainBrand{
	{"hanzo.ai", brandHanzo},
	{"hanzo.id", brandHanzo},
	{"hanzo.network", brandHanzo},
	{"hanzo.chat", brandHanzo},
	{"hanzo.computer", brandHanzo},
	{"lux.network", brandLux},
	{"lux.id", brandLux},
	{"lux.finance", brandLux},
	// The .cloud hosts are where these brands TAKE MONEY (pay.lux.cloud,
	// pay.zoo.cloud). Absent here they fell to defaultBrand() — hanzo — so a Lux
	// customer met Hanzo's brand, IAM app and Square merchant. The fallback is
	// right for an unknown host; the bug was that these were unknown.
	{"lux.cloud", brandLux},
	// lux.tel is the same case one domain later: pay.lux.tel answered with Hanzo's
	// name, Hanzo's logo and Hanzo's IAM app, because it was unknown here and the
	// fallback is hanzo. Every host a brand TAKES MONEY on has to be listed, and
	// the list is the only thing that knows.
	{"lux.tel", brandLux},
	{"zoo.ngo", brandZoo},
	{"zoo.network", brandZoo},
	{"zoo.id", brandZoo},
	{"zoo.cloud", brandZoo},
	{"pars.network", brandPars},
	{"pars.id", brandPars},
	{"pars.vote", brandPars},
}

// A brand carries a logoURL only when it HAS one. The checkout header already
// renders the displayName as a wordmark when this is empty, so a brand without
// a mark reads as itself rather than as a broken image — and, more to the point,
// never as somebody else. Filling these in by pattern would be the way a Hanzo
// logo ends up on a Lux page.
//
// Each was fetched before it was written here. Only Hanzo answered with an
// actual image (200, image/svg+xml); cdn.lux.network 404s and the zoo and pars
// CDNs answer 522. lux.network/logo-white.svg looks like a hit at 200 until you
// read the type — it is 112KB of text/html, the SPA's catch-all serving
// index.html, which the browser would take for an image and fail to draw.
var (
	brandHanzo = brand{slug: "hanzo", displayName: "Hanzo", logoURL: "https://cdn.hanzo.ai/img/logo-white.svg", primaryColor: "#808000", iamIssuer: "https://hanzo.id", iamClientID: "hanzo-app"}
	brandLux   = brand{slug: "lux", displayName: "Lux", primaryColor: "#ffffff", iamIssuer: "https://lux.id", iamClientID: "lux-app"}
	brandZoo   = brand{slug: "zoo", displayName: "Zoo", primaryColor: "#ffffff", iamIssuer: "https://zoolabs.id", iamClientID: "zoo-app"}
	brandPars  = brand{slug: "pars", displayName: "Pars", primaryColor: "#ffffff", iamIssuer: "https://pars.id", iamClientID: "pars-app"}
)

// brandForHost resolves a normalized host to its brand. Unknown hosts fall back
// to the deployment default org (COMMERCE_DEFAULT_ORG, default "hanzo") so a
// same-cluster host that isn't a public brand domain still resolves to the org
// this commerce serves.
func brandForHost(host string) brand {
	for _, bd := range configuredDomains() {
		if host == bd.domain || strings.HasSuffix(host, "."+bd.domain) {
			return bd.brand
		}
	}
	return defaultBrand()
}

// COMMERCE_BRAND_DOMAINS adds paying hosts WITHOUT a release.
//
// The table above is the only thing that knows which hosts a brand takes money on,
// and it is compiled in — so every new one cost a module tag, a dependency bump in
// the host binary, an image build and a pin, to add one line. Meanwhile the host
// answered with the fallback brand, which is somebody else's name and mark on a
// checkout. That price is why lux.cloud and then lux.tel each shipped wrong first.
//
// The form is `domain=brand`, comma or space separated:
//
//	COMMERCE_BRAND_DOMAINS="lux.tel=lux, pay.example.coop=zoo"
//
// Entries are prepended, so configuration BEATS the compiled default and a
// mistake in the table is correctable without a release too. An unknown brand is
// skipped rather than guessed — naming a brand that does not exist should not
// silently mint one.
//
// Read once. This runs on a PUBLIC unauthenticated path (see OrgLoader's warning),
// so it must never do I/O or take a lock per request; parsing an env var at first
// use does neither, and the value cannot change without a restart anyway.
var configuredDomains = sync.OnceValue(func() []domainBrand {
	return append(parseBrandDomains(os.Getenv("COMMERCE_BRAND_DOMAINS")), brandDomains...)
})

// parseBrandDomains reads the `domain=brand` list. Split out so it is testable:
// configuredDomains reads the env exactly once by design, so a test cannot set it.
func parseBrandDomains(raw string) []domainBrand {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	known := map[string]brand{
		brandHanzo.slug: brandHanzo,
		brandLux.slug:   brandLux,
		brandZoo.slug:   brandZoo,
		brandPars.slug:  brandPars,
	}
	extra := make([]domainBrand, 0, 4)
	for _, pair := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' }) {
		d, b, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		d = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(d, ".")))
		br, isKnown := known[strings.ToLower(strings.TrimSpace(b))]
		if d == "" || !isKnown {
			continue
		}
		extra = append(extra, domainBrand{d, br})
	}
	return extra
}

// BrandSlugForHost resolves a customer-facing host to the serving brand's org
// slug — the RECEIVING side of a deposit (whose bank account, whose KMS wire
// path, whose custody vault). Exported for the /v1/billing wire + crypto
// top-up surface, which shares this one host→brand table with the org
// endpoint rather than growing a second.
func BrandSlugForHost(host string) string {
	return brandForHost(host).slug
}

// defaultBrand is the deployment's fallback org. It is brandHanzo unless
// COMMERCE_DEFAULT_ORG names another known brand (so a lux/zoo/pars-only
// commerce deploy can flip its default without code changes).
func defaultBrand() brand {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("COMMERCE_DEFAULT_ORG"))) {
	case "lux":
		return brandLux
	case "zoo":
		return brandZoo
	case "pars":
		return brandPars
	default:
		return brandHanzo
	}
}
