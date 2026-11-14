package square

import (
	"testing"

	"github.com/hanzoai/commerce/payment/processor"
)

// hostileProduction sets every production Square name to a value a sandbox
// read must never return.
func hostileProduction(t *testing.T) {
	t.Helper()
	t.Cleanup(processor.EnvSeed{"SQUARE_ACCESS_TOKEN": "EAAA-prod-hostile", "SQUARE_LOCATION_ID": "PRODLOC-hostile", "SQUARE_APPLICATION_ID": "sq0idp-hostile"}.Apply())
}

// A sandbox read with no sandbox values is unconfigured, never the production
// account.
func TestDeploymentSandboxReadsNoProductionName(t *testing.T) {
	hostileProduction(t)
	t.Cleanup(processor.EnvSeed{"SQUARE_SANDBOX_ACCESS_TOKEN": "", "SQUARE_SANDBOX_LOCATION_ID": "", "SQUARE_SANDBOX_APPLICATION_ID": ""}.Apply())

	if got := Deployment(true); got != (Credentials{}) {
		t.Fatalf("Deployment(sandbox) = %+v with only production names set, want empty", got)
	}
}

// A partial sandbox set stays partial: no production half fills the gap, so a
// sandbox token is never paired with a production location.
func TestDeploymentSandboxPartialStaysPartial(t *testing.T) {
	hostileProduction(t)
	t.Cleanup(processor.EnvSeed{"SQUARE_SANDBOX_ACCESS_TOKEN": "EAAA-sandbox", "SQUARE_SANDBOX_LOCATION_ID": "", "SQUARE_SANDBOX_APPLICATION_ID": ""}.Apply())

	got := Deployment(true)
	if got.AccessToken != "EAAA-sandbox" || got.LocationID != "" || got.ApplicationID != "" {
		t.Fatalf("Deployment(sandbox) = %+v, want the sandbox token and nothing else", got)
	}
}

// Production reads the production names and no sandbox one.
func TestDeploymentProductionReadsProductionNames(t *testing.T) {
	hostileProduction(t)
	t.Cleanup(processor.EnvSeed{"SQUARE_SANDBOX_ACCESS_TOKEN": "EAAA-sandbox", "SQUARE_SANDBOX_LOCATION_ID": "SBXLOC", "SQUARE_SANDBOX_APPLICATION_ID": "sandbox-sq0idb-SBX"}.Apply())

	want := Credentials{AccessToken: "EAAA-prod-hostile", LocationID: "PRODLOC-hostile", ApplicationID: "sq0idp-hostile"}
	if got := Deployment(false); got != want {
		t.Fatalf("Deployment(production) = %+v, want %+v", got, want)
	}
}

func TestSandboxEnv(t *testing.T) {
	for v, want := range map[string]bool{"sandbox": true, "TEST": true, " Sandbox ": true, "production": false, "": false, "live": false} {
		t.Cleanup(processor.EnvSeed{"SQUARE_ENVIRONMENT": v, "SQUARE_APPLICATION_ID": "live-app", "SQUARE_SANDBOX_APPLICATION_ID": "sandbox-app"}.Apply())
		if got := SandboxEnv(); got != want {
			t.Errorf("SandboxEnv(%q) = %v, want %v", v, got, want)
		}
	}
}
