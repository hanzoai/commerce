package billing

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/thirdparty/kms"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// GetPaymentConfig returns the PUBLIC Square config (application id, location id,
// environment) the browser's Web Payments SDK must use to tokenize a card for
// THIS org. It mirrors payment.ProcessorsForOrg's sandbox-vs-production rule
// (org.Live is the authority: a test org → Square sandbox, a live org →
// production) and the same KMS-then-env fallback, so the app id the browser
// tokenizes with always matches the access token commerce will vault/charge
// with. All values are public (safe to expose to the client).
//
//	GET /v1/billing/payment-config
func GetPaymentConfig(c *gin.Context) {
	org := middleware.GetOrganization(c)

	// Best-effort KMS hydrate so an org with its own Square account gets its
	// own app id; the env fallback below covers the platform Square account.
	if v, ok := c.Get("kms"); ok {
		if kmsClient, ok := v.(*kms.CachedClient); ok {
			_ = kms.Hydrate(kmsClient, org)
		}
	}

	sqCfg := org.SquareConfig(!org.Live)
	appID := sqCfg.ApplicationId
	locationID := sqCfg.LocationId
	env := "production"
	if !org.Live {
		env = "sandbox"
	}

	if appID == "" || locationID == "" {
		// Same fallback semantics as payment/orgsetup.go: org.Live decides
		// sandbox vs production; SQUARE_ENVIRONMENT=sandbox can force sandbox
		// for a live org (all-sandbox deployment).
		squareEnv := strings.ToLower(strings.TrimSpace(os.Getenv("SQUARE_ENVIRONMENT")))
		envSandbox := squareEnv == "sandbox" || squareEnv == "test"
		useSandbox := !org.Live || envSandbox

		appID = strings.TrimSpace(os.Getenv("SQUARE_APPLICATION_ID"))
		locationID = strings.TrimSpace(os.Getenv("SQUARE_LOCATION_ID"))
		if useSandbox {
			if a := strings.TrimSpace(os.Getenv("SQUARE_SANDBOX_APPLICATION_ID")); a != "" {
				appID = a
			}
			if l := strings.TrimSpace(os.Getenv("SQUARE_SANDBOX_LOCATION_ID")); l != "" {
				locationID = l
			}
			env = "sandbox"
		} else {
			env = "production"
		}
	}

	c.JSON(200, gin.H{
		"provider":      "square",
		"applicationId": appID,
		"locationId":    locationID,
		"environment":   env,
		"live":          org.Live,
	})
}

type testModeRequest struct {
	TestMode bool `json:"testMode"`
}

// SetOrgTestMode toggles the org between Square test (sandbox) and live mode.
// In test mode (org.Live=false) both the Web Payments SDK (via GetPaymentConfig)
// and commerce vaulting/charging (via payment.ProcessorsForOrg) use Square
// sandbox; live mode uses production. Admin-only — a user must not be able to
// move their own org to sandbox to dodge real charges.
//
//	POST /v1/billing/test-mode   { testMode: bool }
func SetOrgTestMode(c *gin.Context) {
	org := middleware.GetOrganization(c)

	var req testModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonhttp.Fail(c, 400, "invalid request body", err)
		return
	}

	org.Live = !req.TestMode
	if err := org.Update(); err != nil {
		log.Error("Failed to set test mode for org %q: %v", org.Name, err, c)
		jsonhttp.Fail(c, 500, "failed to set test mode", err)
		return
	}

	c.JSON(200, gin.H{
		"orgId":    org.Id(),
		"orgName":  org.Name,
		"live":     org.Live,
		"testMode": !org.Live,
	})
}
