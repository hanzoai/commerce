package stripe

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hanzoai/commerce/payment/processor"
)

var _ processor.PayerProfiles = (*Provider)(nil)

// payerParams are the customer fields of p that are known, in Stripe's form.
// Metadata carries the org and the user so the customer is recognised in the
// Dashboard as a person of an organization, never as the organization's slug.
func payerParams(p processor.Payer) url.Values {
	v := url.Values{}
	set := func(k, val string) {
		if val = strings.TrimSpace(val); val != "" {
			v.Set(k, val)
		}
	}
	set("name", p.Name)
	set("email", p.Email)
	set("phone", p.Phone)
	set("preferred_locales[0]", p.Locale)
	if a := p.Address; !a.Empty() {
		set("address[line1]", a.Line1)
		set("address[line2]", a.Line2)
		set("address[city]", a.City)
		set("address[state]", a.State)
		set("address[postal_code]", a.Postal)
		set("address[country]", strings.ToUpper(a.Country))
	}
	set("metadata[org]", p.Org)
	set("metadata[user]", p.User)
	if p.Org != "" {
		set("description", "Hanzo — "+p.Org)
	}
	return v
}

// CreateCustomerFor makes the payer's customer with everything known. One customer
// per reference, whatever retries: Stripe answers a repeated create under the same
// key with the customer already made.
func (p *Provider) CreateCustomerFor(ctx context.Context, payer processor.Payer, reference string, metadata map[string]interface{}) (string, error) {
	params := payerParams(payer)
	for k, v := range metadata {
		if k == "org" || k == "user" {
			continue
		}
		params.Set("metadata["+k+"]", fmt.Sprintf("%v", v))
	}
	key := ""
	if reference != "" {
		key = "customer:" + reference
	}
	var cust stripeCustomer
	if err := p.postKey(ctx, "/customers", params, key, &cust); err != nil {
		return "", err
	}
	return cust.ID, nil
}

// ProfileCustomer brings the customer up to date with the payer: every field the
// payer states that the customer lacks or holds differently is written, in one
// call, and nothing is written when they already agree.
func (p *Provider) ProfileCustomer(ctx context.Context, customerID string, payer processor.Payer) error {
	var cust stripeCustomer
	if err := p.get(ctx, "/customers/"+url.PathEscape(customerID), nil, &cust); err != nil {
		return err
	}
	want := payerParams(payer)
	have := map[string]string{
		"name": cust.Name, "email": cust.Email, "phone": cust.Phone,
		"metadata[org]": str(cust.Metadata["org"]), "metadata[user]": str(cust.Metadata["user"]),
	}
	if a := cust.Address; a != nil {
		have["address[line1]"], have["address[line2]"], have["address[city]"] = a.Line1, a.Line2, a.City
		have["address[state]"], have["address[postal_code]"], have["address[country]"] = a.State, a.PostalCode, a.Country
	}
	diff := url.Values{}
	for k := range want {
		if k == "preferred_locales[0]" || k == "description" {
			continue
		}
		if want.Get(k) != have[k] {
			diff.Set(k, want.Get(k))
		}
	}
	if len(diff) == 0 {
		return nil
	}
	var out stripeCustomer
	return p.post(ctx, "/customers/"+url.PathEscape(customerID), diff, &out)
}

func str(v interface{}) string {
	s, _ := v.(string)
	return s
}

// completeBilling writes onto a saved card the billing details the card lacks, from
// the customer it belongs to: the issuer's checks (and Radar) read name, email, phone
// and address off the card, and a card the browser tokenized without them carries
// only a postal code.
func (p *Provider) completeBilling(ctx context.Context, customerID, pmID string) {
	var cust stripeCustomer
	if err := p.get(ctx, "/customers/"+url.PathEscape(customerID), nil, &cust); err != nil {
		return
	}
	var pm struct {
		BillingDetails struct {
			Name    string         `json:"name"`
			Email   string         `json:"email"`
			Phone   string         `json:"phone"`
			Address *stripeAddress `json:"address"`
		} `json:"billing_details"`
	}
	if err := p.get(ctx, "/payment_methods/"+url.PathEscape(pmID), nil, &pm); err != nil {
		return
	}
	bd, up := pm.BillingDetails, url.Values{}
	fill := func(k, have, val string) {
		if strings.TrimSpace(have) == "" && strings.TrimSpace(val) != "" {
			up.Set("billing_details["+k+"]", val)
		}
	}
	fill("name", bd.Name, cust.Name)
	fill("email", bd.Email, cust.Email)
	fill("phone", bd.Phone, cust.Phone)
	have := stripeAddress{}
	if bd.Address != nil {
		have = *bd.Address
	}
	if a := cust.Address; a != nil {
		fill("address][line1", have.Line1, a.Line1)
		fill("address][line2", have.Line2, a.Line2)
		fill("address][city", have.City, a.City)
		fill("address][state", have.State, a.State)
		fill("address][postal_code", have.PostalCode, a.PostalCode)
		fill("address][country", have.Country, a.Country)
	}
	if len(up) == 0 {
		return
	}
	var out struct{}
	_ = p.post(ctx, "/payment_methods/"+url.PathEscape(pmID), up, &out)
}
