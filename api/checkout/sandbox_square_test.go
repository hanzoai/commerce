package checkout

import (
	"testing"

	"github.com/hanzoai/commerce/models/organization"
)

// A test-mode org with no Square account of its own, on a deployment with no
// sandbox account, mints no payment link: the production account is never its
// fallback, on the sandbox host or any other.
func TestSquareCheckoutClient_SandboxNeverBorrowsProduction(t *testing.T) {
	t.Setenv("SQUARE_ENVIRONMENT", "production")
	t.Setenv("SQUARE_ACCESS_TOKEN", "EAAA-prod-hostile")
	t.Setenv("SQUARE_LOCATION_ID", "PRODLOC-hostile")
	t.Setenv("SQUARE_SANDBOX_ACCESS_TOKEN", "")
	t.Setenv("SQUARE_SANDBOX_LOCATION_ID", "")

	if _, loc, err := squareCheckoutClientForOrg(&organization.Organization{}); err == nil {
		t.Fatalf("sandbox org given a Square client at %q from production names", loc)
	}

	t.Setenv("SQUARE_SANDBOX_ACCESS_TOKEN", "EAAA-sandbox")
	t.Setenv("SQUARE_SANDBOX_LOCATION_ID", "SBXLOC")
	_, loc, err := squareCheckoutClientForOrg(&organization.Organization{})
	if err != nil || loc != "SBXLOC" {
		t.Fatalf("sandbox org: loc=%q err=%v, want the sandbox location", loc, err)
	}
}
