package payment

import (
	"os"
	"testing"

	"github.com/hanzoai/commerce/models/organization"
)

// SQUARE_ENVIRONMENT is the single authority for the sandbox-vs-production split;
// org.Live is the fallback only when the var is unset or unrecognized.
func TestSquareUseSandbox(t *testing.T) {
	cases := []struct {
		name   string
		env    string
		envSet bool
		live   bool
		want   bool // want sandbox?
	}{
		{"production env overrides live org", "production", true, true, false},
		{"production env overrides test org", "production", true, false, false},
		{"prod alias", "prod", true, false, false},
		{"live alias", "live", true, false, false},
		{"mixed-case PRODUCTION", "PRODUCTION", true, false, false},
		{"sandbox env overrides live org", "sandbox", true, true, true},
		{"sandbox env, test org", "sandbox", true, false, true},
		{"test alias", "test", true, true, true},
		{"padded sandbox", "  sandbox  ", true, true, true},
		{"unset -> live org is production", "", false, true, false},
		{"unset -> test org is sandbox", "", false, false, true},
		{"unrecognized -> fallback !live (live)", "bogus", true, true, false},
		{"unrecognized -> fallback !live (test)", "bogus", true, false, true},
		{"empty-but-set -> fallback !live (live)", "", true, true, false},
		{"whitespace-only -> fallback !live (test)", "   ", true, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envSet {
				t.Setenv("SQUARE_ENVIRONMENT", tc.env)
			} else if v, ok := os.LookupEnv("SQUARE_ENVIRONMENT"); ok {
				os.Unsetenv("SQUARE_ENVIRONMENT")
				t.Cleanup(func() { _ = os.Setenv("SQUARE_ENVIRONMENT", v) })
			}

			org := &organization.Organization{}
			org.Live = tc.live

			if got := SquareUseSandbox(org); got != tc.want {
				t.Fatalf("SquareUseSandbox(live=%v, env=%q set=%v) = %v, want %v", tc.live, tc.env, tc.envSet, got, tc.want)
			}

			wantEnv := "production"
			if tc.want {
				wantEnv = "sandbox"
			}
			if got := SquareEnvironment(org); got != wantEnv {
				t.Fatalf("SquareEnvironment = %q, want %q", got, wantEnv)
			}
		})
	}
}
