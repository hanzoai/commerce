package billing

import (
	"os"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// The billing SUBJECT is the wallet money lands in and the gate reads out of. It is
// NOT always the org: hanzoai/ai (object/billing_subject.go) already resolves a
// personal-billing org's members to a per-user subject, and the LLM gate reads that
// subject. Commerce keyed every user-scoped write on the org slug instead, so the
// welcome credit (and any top-up) landed in the org's pooled wallet while the gate
// read the person's — a customer topped up one key and read another.
//
// One rule, mirrored from ai so the two can never drift:
//
//	personal-billing org (PERSONAL_BILLING_ORGS, default "hanzo") → "<org>/<user>"
//	pooled org (a real company)                                    → "<org>"
//
// A person therefore has a personal balance and a personal plan; an org pays for what
// its applications and service keys spend.

var (
	personalBillingOrgsOnce sync.Once
	personalBillingOrgs     map[string]struct{}
)

func loadPersonalBillingOrgs() map[string]struct{} {
	personalBillingOrgsOnce.Do(func() {
		raw := os.Getenv("PERSONAL_BILLING_ORGS")
		if strings.TrimSpace(raw) == "" {
			raw = "hanzo" // same fail-safe default as ai; keep them identical
		}
		m := make(map[string]struct{})
		for _, o := range strings.Split(raw, ",") {
			if o = strings.ToLower(strings.TrimSpace(o)); o != "" {
				m[o] = struct{}{}
			}
		}
		personalBillingOrgs = m
	})
	return personalBillingOrgs
}

// IsPersonalBillingOrg reports whether members of org hold individual balances rather
// than sharing the org's pooled one.
func IsPersonalBillingOrg(org string) bool {
	_, ok := loadPersonalBillingOrgs()[strings.ToLower(strings.TrimSpace(org))]
	return ok
}

// BillingSubjectFor is the canonical subject for an (org, user) identity. It must
// produce byte-for-byte what ai's BillingSubject produces, or deposits and the gate
// diverge.
func BillingSubjectFor(org, user string) string {
	org = strings.ToLower(strings.TrimSpace(org))
	if org == "" {
		return ""
	}
	if !IsPersonalBillingOrg(org) {
		return org
	}
	user = strings.ToLower(strings.TrimSpace(user))
	if user == "" {
		return org // an org-owned principal (application / service key) pays from the pool
	}
	// ai keys the per-user subject as "<org>/<name>"; a caller may already hand us the
	// fully-qualified id, so never double-prefix it.
	if strings.HasPrefix(user, org+"/") {
		return user
	}
	return org + "/" + user
}

// userBillingKey resolves the subject for a USER-scoped billing call: the person when
// the org bills personally, the org otherwise. Identity comes from the gateway-injected
// X-User-Id header (edgeauth mints it); the org still comes from the resolved org.
func userBillingKey(c *gin.Context) string {
	org := orgBillingKey(c)
	if org == "" {
		return ""
	}
	return BillingSubjectFor(org, c.GetHeader("X-User-Id"))
}

// resetPersonalBillingOrgsForTest re-reads PERSONAL_BILLING_ORGS. The cache is a
// sync.Once so a test that sets the env after first use would otherwise read a stale map.
func resetPersonalBillingOrgsForTest() {
	personalBillingOrgsOnce = sync.Once{}
	personalBillingOrgs = nil
}
