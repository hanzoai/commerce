package billing

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/util/test/ae"
)

// An organization's order switches on no automatic reload of its own: a seat plan
// whose row carries no reload leaves the organization's rule as it was, and only a
// row that carries one switches it on, at the row's figures, as every purchase of
// such a plan does.
func TestAnOrderArmsNoReloadUnlessItsPlanCarriesOne(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	for _, c := range []struct {
		org, slug string
		reload    *plan.Reload
	}{
		{"so-plain", "so-ent-plain", nil},
		{"so-reload", "so-ent-reload", &plan.Reload{ThresholdCents: 1000, AmountCents: 5000}},
	} {
		org := moneyOrg(c.org)
		withFakeSquare(t, squareMock("cust_"+c.org, "ccof_"+c.org, "sqpay_"+c.org))
		row := orderRow(t, ctx, c.slug, 20, 500)
		if c.reload != nil {
			row.Limits.Reload = c.reload
			if err := row.Update(); err != nil {
				t.Fatalf("give %s a reload: %v", c.slug, err)
			}
		}

		body := `{"sourceId":"cnon:ok","planId":"` + c.slug + `","quantity":20,"interval":"year","balanceCents":40000}`
		r := invokeSubscribeCard(org, ctx, body, nil)
		if r.StatusCode != http.StatusCreated {
			t.Fatalf("%s: status=%d body=%s, want 201", c.slug, r.StatusCode, bodyText(r))
		}
		raw, _ := io.ReadAll(r.Body)
		var sale struct {
			Reload *Arming `json:"reload"`
		}
		if err := json.Unmarshal(raw, &sale); err != nil {
			t.Fatalf("%s: receipt %s: %v", c.slug, raw, err)
		}
		rule := loadAutoRecharge(datastore.New(org.Namespaced(ctx)), org.Name)

		if c.reload == nil {
			if sale.Reload != nil || (rule != nil && rule.Enabled) {
				t.Fatalf("%s: an order of a plan with no reload answered %+v and left the rule %+v, want none switched on", c.slug, sale.Reload, rule)
			}
			continue
		}
		if sale.Reload == nil || sale.Reload.State != "armed" || rule == nil || !rule.Enabled ||
			rule.ThresholdCents != 1000 || rule.AmountCents != 5000 {
			t.Fatalf("%s: an order of a plan with a reload answered %+v and left the rule %+v, want it on at $50 below $10", c.slug, sale.Reload, rule)
		}
	}
}
