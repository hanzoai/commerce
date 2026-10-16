package square

import "testing"

// hostileProduction sets every production Square name to a value a sandbox
// read must never return.
func hostileProduction(t *testing.T) {
	t.Helper()
	t.Setenv("SQUARE_ACCESS_TOKEN", "EAAA-prod-hostile")
	t.Setenv("SQUARE_LOCATION_ID", "PRODLOC-hostile")
	t.Setenv("SQUARE_APPLICATION_ID", "sq0idp-hostile")
}

// A sandbox read with no sandbox values is unconfigured, never the production
// account.
func TestFromEnvSandboxReadsNoProductionName(t *testing.T) {
	hostileProduction(t)
	t.Setenv("SQUARE_SANDBOX_ACCESS_TOKEN", "")
	t.Setenv("SQUARE_SANDBOX_LOCATION_ID", "")
	t.Setenv("SQUARE_SANDBOX_APPLICATION_ID", "")

	if got := FromEnv(true); got != (Credentials{}) {
		t.Fatalf("FromEnv(sandbox) = %+v with only production names set, want empty", got)
	}
}

// A partial sandbox set stays partial: no production half fills the gap, so a
// sandbox token is never paired with a production location.
func TestFromEnvSandboxPartialStaysPartial(t *testing.T) {
	hostileProduction(t)
	t.Setenv("SQUARE_SANDBOX_ACCESS_TOKEN", "EAAA-sandbox")
	t.Setenv("SQUARE_SANDBOX_LOCATION_ID", "")
	t.Setenv("SQUARE_SANDBOX_APPLICATION_ID", "")

	got := FromEnv(true)
	if got.AccessToken != "EAAA-sandbox" || got.LocationID != "" || got.ApplicationID != "" {
		t.Fatalf("FromEnv(sandbox) = %+v, want the sandbox token and nothing else", got)
	}
}

// Production reads the production names and no sandbox one.
func TestFromEnvProductionReadsProductionNames(t *testing.T) {
	hostileProduction(t)
	t.Setenv("SQUARE_SANDBOX_ACCESS_TOKEN", "EAAA-sandbox")
	t.Setenv("SQUARE_SANDBOX_LOCATION_ID", "SBXLOC")
	t.Setenv("SQUARE_SANDBOX_APPLICATION_ID", "sandbox-sq0idb-SBX")

	want := Credentials{AccessToken: "EAAA-prod-hostile", LocationID: "PRODLOC-hostile", ApplicationID: "sq0idp-hostile"}
	if got := FromEnv(false); got != want {
		t.Fatalf("FromEnv(production) = %+v, want %+v", got, want)
	}
}

func TestSandboxEnv(t *testing.T) {
	for v, want := range map[string]bool{"sandbox": true, "TEST": true, " Sandbox ": true, "production": false, "": false, "live": false} {
		t.Setenv("SQUARE_ENVIRONMENT", v)
		if got := SandboxEnv(); got != want {
			t.Errorf("SandboxEnv(%q) = %v, want %v", v, got, want)
		}
	}
}
