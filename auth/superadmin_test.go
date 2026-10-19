package auth

import (
	"testing"

	"github.com/hanzoai/authz"
)

// TestIAMClaims_IsSuperAdmin pins the canonical platform-sudo predicate shared by
// every cross-org gate (edge billing ?org override, checkout tenant admin, the
// money-mint gate): the identity edge's verdict, Sudo, and nothing else. No org
// field decides it — not the `owner` claim (the minting application's org), not
// the org the request acts in, in any case or padding — and an org owner's
// org-level IsAdmin never does.
func TestIAMClaims_IsSuperAdmin(t *testing.T) {
	cases := []struct {
		name   string
		claims *IAMClaims
		want   bool
	}{
		{"nil", nil, false},
		{"the edge's verdict", &IAMClaims{Owner: "admin", Sudo: true}, true},
		{"the edge's verdict, acting in a tenant", &IAMClaims{Owner: "victim", Sudo: true}, true},
		{"owner admin, no verdict", &IAMClaims{Owner: "admin"}, false},
		{"owner Admin, no verdict", &IAMClaims{Owner: "Admin"}, false},
		{"owner padded, no verdict", &IAMClaims{Owner: " admin "}, false},
		{"org-owner isAdmin is NOT SuperAdmin (maxpower)", &IAMClaims{Owner: "maxpower", IsAdmin: true}, false},
		{"platform org is NOT the admin org", &IAMClaims{Owner: "platform", IsAdmin: true}, false},
		{"plain user", &IAMClaims{Owner: "hanzo"}, false},
		{"empty owner", &IAMClaims{Owner: ""}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.claims.IsSuperAdmin(); got != tc.want {
				t.Fatalf("%s: IsSuperAdmin()=%v want %v", tc.name, got, tc.want)
			}
		})
	}
}

// Authz is what the standalone edge asks authz.Claims.Sudo of, so the verdict it
// writes is the one predicate's: a PERSON whose own org — orgs[0] — is "admin",
// verbatim. The `owner` claim decides nothing, and a program never qualifies.
func TestIAMClaims_AuthzIsTheOnePredicate(t *testing.T) {
	m := func(orgs ...string) []authz.Membership {
		out := make([]authz.Membership, len(orgs))
		for i, o := range orgs {
			out[i] = authz.Membership{Org: o, Role: authz.Owner}
		}
		return out
	}
	for _, tc := range []struct {
		name   string
		claims *IAMClaims
		want   bool
	}{
		{"person in admin", &IAMClaims{Owner: "hanzo", Orgs: m("admin")}, true},
		{"person in hanzo holding an admin membership", &IAMClaims{Owner: "admin", Orgs: m("hanzo", "admin")}, false},
		{"person in an org spelled Admin", &IAMClaims{Owner: "Admin", Orgs: m("Admin")}, false},
		{"admin-org application (no memberships)", &IAMClaims{Owner: "admin"}, false},
		{"admin-org application that says so", &IAMClaims{Owner: "admin", Type: authz.Program, Orgs: m("admin")}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.claims.Authz().Sudo(); got != tc.want {
				t.Fatalf("Authz().Sudo()=%v want %v", got, tc.want)
			}
		})
	}
}
