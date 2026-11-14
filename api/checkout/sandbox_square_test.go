package checkout

import (
	"github.com/hanzoai/commerce/payment/processor"
	"testing"

	"github.com/hanzoai/commerce/models/organization"
)

// A test-mode org with no Square account of its own, on a deployment with no
// sandbox account, mints no payment link: the production account is never its
// fallback, on the sandbox host or any other.
func TestSquareCheckoutClient_SandboxNeverBorrowsProduction(t *testing.T) {
	t.Cleanup(processor.EnvSeed{"SQUARE_ENVIRONMENT": "production", "SQUARE_ACCESS_TOKEN": "EAAA-prod-hostile", "SQUARE_LOCATION_ID": "PRODLOC-hostile", "SQUARE_SANDBOX_ACCESS_TOKEN": "", "SQUARE_SANDBOX_LOCATION_ID": ""}.Apply())

	if _, loc, err := squareCheckoutClientForOrg(&organization.Organization{}); err == nil {
		t.Fatalf("sandbox org given a Square client at %q from production names", loc)
	}

	t.Cleanup(processor.EnvSeed{"SQUARE_SANDBOX_ACCESS_TOKEN": "EAAA-sandbox", "SQUARE_SANDBOX_LOCATION_ID": "SBXLOC"}.Apply())
	_, loc, err := squareCheckoutClientForOrg(&organization.Organization{})
	if err != nil || loc != "SBXLOC" {
		t.Fatalf("sandbox org: loc=%q err=%v, want the sandbox location", loc, err)
	}
}
