package square

import (
	"strings"

	"github.com/hanzoai/commerce/payment/processor"
)

// Credentials is the deployment's own Square account of one kind, as the routing
// policy's square accounts give it.
type Credentials struct {
	AccessToken   string
	LocationID    string
	ApplicationID string
}

// Deployment is the deployment's Square credentials of one kind: the "sandbox" account for
// sandbox, the "live" account for production. The access token is read from KMS at the
// account's secret path; the location and application ids are plain settings.
//
// The two sets never mix. A sandbox read consults no production account, so a sandbox set
// left empty is an unconfigured processor, never the production account: a sandbox
// request that fell back to the live token would send a real merchant's token to Square's
// sandbox host, or, with the sandbox token and the production location, pair two
// accounts' halves.
func Deployment(sandbox bool) Credentials {
	id := "live"
	if sandbox {
		id = "sandbox"
	}
	a := processor.AccountOf(processor.Square, id)
	return Credentials{
		AccessToken:   strings.TrimSpace(a.Secret("accessToken")),
		LocationID:    strings.TrimSpace(a.Setting("locationId")),
		ApplicationID: strings.TrimSpace(a.Setting("applicationId")),
	}
}

// SandboxEnv reports whether the deployment's own Square account is the sandbox one: the
// account the policy marks as the deployment's (setting deployment=true), else production.
func SandboxEnv() bool { return Active().ID == "sandbox" }

// Active is the deployment's own Square account: the one marked deployment=true, else "live".
func Active() processor.Account {
	for _, id := range []string{"sandbox", "live"} {
		if a := processor.AccountOf(processor.Square, id); a.Setting("deployment") == "true" {
			return a
		}
	}
	return processor.AccountOf(processor.Square, "live")
}
