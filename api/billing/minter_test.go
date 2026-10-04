package billing

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/balancetransaction"
	"github.com/hanzoai/commerce/models/creditgrant"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
)

// Every privileged money act a SuperAdmin takes — a mint, a grant, a void, a
// balance adjustment, a comp — writes who took it onto the row it writes.
func TestPrivilegedMoneyActsRecordWhoTookThem(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := organization.New(datastore.New(ctx))
	org.Name = "acme-audit"
	org.Live = true
	const who = "z@hanzo.ai"
	app := engineWithSeed(func(c *zip.Ctx) {
		c.Locals("iam_authenticated", true)
		c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
		c.Locals("iam_claims", &auth.IAMClaims{Owner: "admin", Email: who})
		c.Locals("organization", org)
		c.SetContext(ctx)
	})
	post := func(path, body string, hdr ...string) map[string]any {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		for i := 0; i+1 < len(hdr); i += 2 {
			req.Header.Set(hdr[i], hdr[i+1])
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode >= 300 {
			t.Fatalf("POST %s: %d %s", path, resp.StatusCode, bodyOf(resp))
		}
		return jsonBody(t, resp)
	}
	db := datastore.New(org.Namespaced(ctx))

	g := post("/v1/billing/credits", `{"userId":"acme-audit/alice","amountCents":100}`)
	grantRow := creditgrant.New(db)
	if err := grantRow.GetById(g["id"].(string)); err != nil {
		t.Fatal(err)
	}
	if grantRow.Metadata["mintedBy"] != who {
		t.Errorf("POST /credits grant metadata %v; want mintedBy %s", grantRow.Metadata, who)
	}
	post("/v1/billing/credits/"+grantRow.Id()+"/void", `{}`)
	if err := grantRow.GetById(grantRow.Id()); err != nil {
		t.Fatal(err)
	}
	if grantRow.Metadata["voidedBy"] != who {
		t.Errorf("void metadata %v; want voidedBy %s", grantRow.Metadata, who)
	}

	for _, tc := range []struct{ path, body, key string }{
		{"/v1/billing/credit", `{"org":"acme-audit","amountCents":100,"reason":"comp"}`, ""},
		{"/v1/billing/deposit", `{"user":"acme-audit/alice","amount":100}`, "settle-1"},
	} {
		var out map[string]any
		if tc.key != "" {
			out = post(tc.path, tc.body, "X-Idempotency-Key", tc.key)
		} else {
			out = post(tc.path, tc.body)
		}
		id, _ := out["id"].(string)
		if id == "" {
			id, _ = out["transactionId"].(string)
		}
		tx := transaction.New(db)
		if err := tx.GetById(id); err != nil {
			t.Fatalf("%s row %q: %v", tc.path, id, err)
		}
		if tx.Metadata["mintedBy"] != who {
			t.Errorf("%s transaction metadata %v; want mintedBy %s", tc.path, tx.Metadata, who)
		}
	}

	adj := post("/v1/billing/customer-balance/adjustments", `{"customerId":"acme-audit/alice","amount":100}`)
	bt := balancetransaction.New(db)
	if err := bt.GetById(adj["id"].(string)); err != nil {
		t.Fatal(err)
	}
	if bt.Metadata["by"] != who {
		t.Errorf("adjustment metadata %v; want by %s", bt.Metadata, who)
	}

	sub := post("/v1/billing/subscriptions", `{"userId":"acme-audit","planId":"max-5x","metadata":{"compedBy":"forged"}}`)
	row := subscription.New(db)
	if err := row.GetById(sub["id"].(string)); err != nil {
		t.Fatal(err)
	}
	if row.Metadata["compedBy"] != who {
		t.Errorf("comp subscription metadata %v; want compedBy %s", row.Metadata, who)
	}
}
