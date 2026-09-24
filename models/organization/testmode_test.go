package organization

import (
	"sync"
	"testing"
)

// The org record is the ONLY authority for the test/live (sandbox vs production)
// split. Fail-closed is per tenant: production requires the org's own Live flag.
func TestOrganization_TestMode(t *testing.T) {
	cases := []struct {
		name string
		live bool
		want bool // want test mode (sandbox)?
	}{
		{"live org transacts in production", true, false},
		{"test org transacts in sandbox", false, true},
		{"zero-value org fails closed to sandbox", false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			org := Organization{}
			org.Live = tc.live

			if got := org.TestMode(); got != tc.want {
				t.Fatalf("TestMode(live=%v) = %v, want %v", tc.live, got, tc.want)
			}
			wantEnv := "production"
			if tc.want {
				wantEnv = "sandbox"
			}
			if got := org.SquareEnvironment(); got != wantEnv {
				t.Fatalf("SquareEnvironment = %q, want %q", got, wantEnv)
			}
		})
	}
}

// The regression this file exists for. SQUARE_ENVIRONMENT used to override every
// org, which meant a pod served exactly one mode and each tenant's own flag was
// dead. No environment variable may decide this again — otherwise "one stateless
// replica serves every tenant" quietly stops being true.
func TestOrganization_TestMode_IgnoresDeploymentEnv(t *testing.T) {
	live := Organization{}
	live.Live = true
	test := Organization{}

	for _, v := range []string{"production", "prod", "live", "sandbox", "test", "", "   ", "produciton", "bogus"} {
		t.Run("SQUARE_ENVIRONMENT="+v, func(t *testing.T) {
			t.Setenv("SQUARE_ENVIRONMENT", v)
			if live.TestMode() {
				t.Errorf("live org became test mode under SQUARE_ENVIRONMENT=%q — the env is deciding again", v)
			}
			if !test.TestMode() {
				t.Errorf("test org became production under SQUARE_ENVIRONMENT=%q — the env is deciding again", v)
			}
		})
	}
}

// Multi-tenancy, stated as a property: two orgs resolved in the SAME process get
// their OWN environments. This is what a single horizontally-scaled deployment
// requires, and what the deployment-wide gate made impossible.
func TestOrganization_TestMode_PerTenantInOneProcess(t *testing.T) {
	t.Setenv("SQUARE_ENVIRONMENT", "sandbox") // hostile: the old authority says sandbox

	live := Organization{}
	live.Live = true
	sandbox := Organization{}

	if live.SquareEnvironment() != "production" || sandbox.SquareEnvironment() != "sandbox" {
		t.Fatalf("two tenants did not resolve independently: live=%q sandbox=%q",
			live.SquareEnvironment(), sandbox.SquareEnvironment())
	}

	// And concurrently, because replicas serve tenants in parallel: resolution
	// must be pure, holding no process-wide state that interleaving could corrupt.
	var wg sync.WaitGroup
	errs := make(chan string, 200)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				if live.SquareEnvironment() != "production" {
					errs <- "live org resolved non-production under concurrency"
				}
				return
			}
			if sandbox.SquareEnvironment() != "sandbox" {
				errs <- "test org resolved non-sandbox under concurrency"
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}
