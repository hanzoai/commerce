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

// An organization's order switches on the reload its plan switches on, as every
// paid purchase does (planReload): the row's own figures, the default for a row
// that states none, and nothing for a row that switches it off.
func TestAnOrderArmsNoReloadUnlessItsPlanCarriesOne(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	for _, c := range []struct {
		org, slug string
		reload    *plan.Reload
	}{
		{"so-plain", "so-ent-plain", &plan.Reload{}},
		{"so-reload", "so-ent-reload", &plan.Reload{ThresholdCents: 2000, AmountCents: 7000}},
		{"so-default", "so-ent-default", nil},
	} {
		org := moneyOrg(c.org)
		withFakeSquare(t, squareMock("cust_"+c.org, "ccof_"+c.org, "sqpay_"+c.org))
		row := orderRow(t, ctx, c.slug, 20, 500)
		want := DefaultReload
		if c.reload != nil {
			want = *c.reload
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

		if want.AmountCents == 0 {
			if sale.Reload != nil || (rule != nil && rule.Enabled) {
				t.Fatalf("%s: an order of a plan with no reload answered %+v and left the rule %+v, want none switched on", c.slug, sale.Reload, rule)
			}
			continue
		}
		if sale.Reload == nil || sale.Reload.State != "armed" || rule == nil || !rule.Enabled ||
			rule.ThresholdCents != want.ThresholdCents || rule.AmountCents != want.AmountCents {
			t.Fatalf("%s: an order of a plan with a reload answered %+v and left the rule %+v, want it on at %+v", c.slug, sale.Reload, rule, want)
		}
	}
}
