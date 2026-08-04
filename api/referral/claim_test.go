package referral

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/creditgrant"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/referral"
	"github.com/hanzoai/commerce/models/referrer"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// Claiming a referral code MINTS NOTHING. It used to write two CreditGrants —
// a code-driven, self-service credit mint. The reward is earned instead, and
// accrues as a payable when the referee actually spends.
func TestClaimReferral_MintsNoCredit(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	const ns = "demint"

	ctx := nscontext.WithNamespace(context.Background(), ns)
	db := datastore.New(ctx)

	ref := referrer.New(db)
	ref.Code = "ABCD1234"
	ref.UserId = "referrer-1"
	ref.AffiliateId = "aff-1"
	if err := ref.Create(); err != nil {
		t.Fatalf("create referrer: %v", err)
	}

	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Post("/referral/claim", func(c *zip.Ctx) error {
		org := &organization.Organization{}
		org.Name = ns
		c.Locals("organization", org)
		c.SetContext(ctx)
		c.Locals("iam_authenticated", true)
		c.Locals("iam_claims", &auth.IAMClaims{Owner: ns})
		return c.Next()
	}, ClaimReferral)

	body, _ := json.Marshal(map[string]string{"code": "ABCD1234", "userId": "referee-1"})
	req := httptest.NewRequest(http.MethodPost, "/referral/claim", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Fiber().Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 201 {
		t.Fatalf("claim: got %d: %s", resp.StatusCode, out)
	}

	// No credit was minted, for either party.
	grants := make([]*creditgrant.CreditGrant, 0)
	if _, err := creditgrant.Query(db).GetAll(&grants); err != nil {
		t.Fatalf("query grants: %v", err)
	}
	if len(grants) != 0 {
		t.Errorf("claim minted %d credit grants, want 0", len(grants))
	}
	if bytes.Contains(out, []byte("creditGranted")) {
		t.Errorf("response still advertises minted credit: %s", out)
	}

	// The relationship IS recorded — that is what makes the later accrual possible.
	refs := make([]*referral.Referral, 0)
	if _, err := referral.Query(db).GetAll(&refs); err != nil {
		t.Fatalf("query referrals: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("referral records: got %d, want 1", len(refs))
	}
}
