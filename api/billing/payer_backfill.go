package billing

import (
	"context"
	"fmt"
	"strings"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/payment/processor"
)

// Who is the person behind a billing subject, from the identity provider: their
// real name and email, and false when none is known. The host supplies it; commerce
// holds no identity of its own.
type Who func(ctx context.Context, subject string) (name, email string, ok bool)

// BackfillRow is one processor customer the backfill looked at.
type BackfillRow struct {
	Processor    string `json:"processor"`
	CustomerID   string `json:"customerId"`
	Subject      string `json:"subject"`
	CurrentName  string `json:"currentName"`
	CurrentEmail string `json:"currentEmail"`
	ProposedName string `json:"proposedName"`
	ProposedMail string `json:"proposedEmail"`
	Written      bool   `json:"written"`
}

// BackfillPayers finds the customers commerce made for org's saved cards whose name
// is the billing subject or the org's slug (a name no issuer accepts) and proposes
// the person's own name and email from who. With write false it only reads and
// reports. With write true it brings each listed customer up to date through the
// processor's profile call, which writes nothing for one that already agrees, so it
// is safe to run again. Customers are visited once however many cards they hold.
func BackfillPayers(ctx context.Context, org *organization.Organization, who Who, write bool) ([]BackfillRow, error) {
	if org == nil {
		return nil, fmt.Errorf("payer backfill: no organization")
	}
	db := datastore.New(org.Namespaced(ctx))
	rows := make([]*paymentmethod.PaymentMethod, 0)
	if _, err := paymentmethod.Query(db).Ancestor(db.NewKey("synckey", "", 1, nil)).Filter("Type=", "card").GetAll(&rows); err != nil {
		return nil, err
	}
	reg := processorsForOrg(org)
	seen := map[string]bool{}
	var out []BackfillRow
	for _, pm := range rows {
		id := customerIDOf(pm)
		key := string(providerOf(pm)) + "/" + id
		if id == "" || seen[key] {
			continue
		}
		seen[key] = true
		cp, err := reg.Existing(ctx, providerOf(pm))
		if err != nil {
			continue
		}
		cust, ok := cp.(interface {
			GetCustomer(context.Context, string) (map[string]interface{}, error)
		})
		if !ok {
			continue
		}
		got, err := cust.GetCustomer(ctx, id)
		if err != nil {
			continue
		}
		name, _ := got["name"].(string)
		email, _ := got["email"].(string)
		subject := pm.CustomerId
		if !isBillingKey(name, subject, org.Name) {
			continue
		}
		pname, pmail, known := who(ctx, subject)
		row := BackfillRow{Processor: string(providerOf(pm)), CustomerID: id, Subject: subject, CurrentName: name, CurrentEmail: email}
		if known {
			row.ProposedName, row.ProposedMail = pname, pmail
		}
		if write && known {
			if pp, ok := cp.(processor.PayerProfiles); ok {
				if err := pp.ProfileCustomer(ctx, id, processor.Payer{Name: pname, Email: pmail, Org: org.Name, User: strings.ToLower(subject)}); err == nil {
					row.Written = true
				}
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// isBillingKey reports whether name is a billing key and not a person's name: the
// subject, the org's slug, or the subject's last path segment.
func isBillingKey(name, subject, orgName string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return false
	}
	s := strings.ToLower(strings.TrimSpace(subject))
	last := s
	if i := strings.LastIndex(s, "/"); i >= 0 {
		last = s[i+1:]
	}
	return n == s || n == last || n == strings.ToLower(orgName)
}
