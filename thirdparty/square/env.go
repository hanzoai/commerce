package square

import (
	"os"
	"strings"
)

// Credentials is the deployment's own Square account of one kind, as its
// environment names it.
type Credentials struct {
	AccessToken   string
	LocationID    string
	ApplicationID string
}

// FromEnv reads the deployment's Square credentials of one kind: the
// SQUARE_SANDBOX_* names for sandbox, the SQUARE_* names for production.
//
// The two sets never mix. A sandbox read consults no production name, so a
// sandbox set left empty is an unconfigured processor, never the production
// account: a sandbox request that fell back to SQUARE_ACCESS_TOKEN would send
// a real merchant's token to Square's sandbox host, or, with the sandbox token
// and the production location, pair two accounts' halves.
func FromEnv(sandbox bool) Credentials {
	if sandbox {
		return Credentials{
			AccessToken:   envValue("SQUARE_SANDBOX_ACCESS_TOKEN"),
			LocationID:    envValue("SQUARE_SANDBOX_LOCATION_ID"),
			ApplicationID: envValue("SQUARE_SANDBOX_APPLICATION_ID"),
		}
	}
	return Credentials{
		AccessToken:   envValue("SQUARE_ACCESS_TOKEN"),
		LocationID:    envValue("SQUARE_LOCATION_ID"),
		ApplicationID: envValue("SQUARE_APPLICATION_ID"),
	}
}

// SandboxEnv reports whether the deployment's SQUARE_ENVIRONMENT names
// Square's sandbox ("sandbox" or "test", any case).
func SandboxEnv() bool {
	env := strings.ToLower(envValue("SQUARE_ENVIRONMENT"))
	return env == "sandbox" || env == "test"
}

func envValue(name string) string { return strings.TrimSpace(os.Getenv(name)) }
