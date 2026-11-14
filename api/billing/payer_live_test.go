package billing

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/types"
)

func ada(org, user string) PayerIn {
	return PayerIn{
		Name: "Ada Lovelace", Email: "ada@example.com", Phone: "+14155550123",
		Address: &types.Address{Line1: "1 Market St", City: "San Francisco", State: "CA", PostalCode: "94115", Country: "US"},
	}
}

func nested(m map[string]any, keys ...string) string {
	var cur any = m
	for _, k := range keys {
		mm, _ := cur.(map[string]any)
		cur = mm[k]
	}
	s, _ := cur.(string)
	return s
}

func dump(t *testing.T, label string, v map[string]any, keys ...string) {
	t.Helper()
	out := map[string]any{}
	for _, k := range keys {
		out[k] = v[k]
	}
	b, _ := json.Marshal(out)
	t.Logf("%s %s", label, b)
}

// A saved card makes ONE Stripe customer named for the PERSON, with their email,
// phone and billing address, and the org and user in metadata: not the org's slug.
// The card carries the same details, which is what the issuer reads.
func TestLiveStripe_TheCustomerAndTheCardNameThePayer(t *testing.T) {
	l := liveSetup(t)
	db := datastore.New(l.org.Namespaced(*l.ctx))
	cp := mustVault(t, l)

	payer := payerFor(l.org, l.subject, ada(l.org.Name, l.subject))
	pm, created, err := saveCard(context.Background(), db, cp, l.subject, payer, "pm_card_visa")
	if err != nil || !created {
		t.Fatalf("saving the card: %v created=%v", err, created)
	}
	custID := metaStr(pm, "customerId")
	cust := l.call("GET", "/customers/"+custID, nil)
	dump(t, "AFTER customer", cust, "name", "email", "phone", "address", "description", "metadata")
	if cust["name"] != "Ada Lovelace" || cust["email"] != "ada@example.com" || cust["phone"] != "+14155550123" {
		t.Fatalf("the customer is %v / %v / %v; want the person", cust["name"], cust["email"], cust["phone"])
	}
	if strings.EqualFold(str(cust["name"]), l.subject) || nested(cust, "address", "line1") != "1 Market St" ||
		nested(cust, "address", "postal_code") != "94115" || nested(cust, "address", "country") != "US" {
		t.Fatalf("the customer's address = %v", cust["address"])
	}
	if nested(cust, "metadata", "org") != l.org.Name || nested(cust, "metadata", "user") != strings.ToLower(l.subject) {
		t.Fatalf("the customer's metadata = %v", cust["metadata"])
	}

	card := l.call("GET", "/payment_methods/"+pm.ProviderRef, nil)
	bd, _ := card["billing_details"].(map[string]any)
	dump(t, "AFTER payment_method billing_details", map[string]any{"billing_details": bd}, "billing_details")
	if bd["name"] != "Ada Lovelace" || bd["email"] != "ada@example.com" || bd["phone"] != "+14155550123" ||
		nested(bd, "address", "line1") != "1 Market St" || nested(bd, "address", "country") != "US" {
		t.Fatalf("the card's billing_details = %v; want the payer's", bd)
	}

	// A charge on that card: the receipt goes to the payer, the statement says who,
	// the description says what, the metadata says whose.
	res, cerr := chargeSavedCard(context.Background(), processorsForOrg(l.org), pm, 1500, currency.USD, "payer-live-"+l.run, "Subscription Team — first period, 5 seats, Nov 13 2026", true)
	if cerr != nil || res == nil || !res.Success {
		t.Fatalf("charge: %v %+v", cerr, res)
	}
	pi := l.call("GET", "/payment_intents/"+res.TransactionID, nil)
	dump(t, "AFTER payment_intent", pi, "description", "receipt_email", "statement_descriptor_suffix", "metadata", "customer")
	if pi["receipt_email"] != "ada@example.com" || pi["statement_descriptor_suffix"] != processor.StatementSuffix ||
		pi["customer"] != custID || !strings.Contains(str(pi["description"]), "5 seats") {
		t.Fatalf("the payment is %v / %v / %v / %v", pi["receipt_email"], pi["statement_descriptor_suffix"], pi["customer"], pi["description"])
	}
	if nested(pi, "metadata", "org") != l.org.Name || nested(pi, "metadata", "user") != strings.ToLower(l.subject) || nested(pi, "metadata", "hanzo_subject") != l.subject {
		t.Fatalf("the payment's metadata = %v", pi["metadata"])
	}
}

// A customer made before the processor was told who it is gets named at its next
// checkout: one customer, updated in place, and nothing written when it already agrees.
func TestLiveStripe_ACustomerMadeNamelessIsNamedAtItsNextCheckout(t *testing.T) {
	l := liveSetup(t)
	db := datastore.New(l.org.Namespaced(*l.ctx))
	cp := mustVault(t, l)

	// How it was: named for the org slug, nothing else.
	old := l.call("POST", "/customers", url.Values{"name": {"github"}, "metadata[reference]": {"hanzo/" + l.subject + "-old"}})
	profile := payerFor(l.org, l.subject, ada(l.org.Name, l.subject))
	if err := cp.(processor.PayerProfiles).ProfileCustomer(context.Background(), str(old["id"]), profile); err != nil {
		t.Fatal(err)
	}
	got := l.call("GET", "/customers/"+str(old["id"]), nil)
	dump(t, "AFTER (was name=github, no email/address) customer", got, "name", "email", "phone", "address", "metadata")
	if got["name"] != "Ada Lovelace" || got["email"] != "ada@example.com" || nested(got, "address", "line1") != "1 Market St" {
		t.Fatalf("the customer was not brought up to date: %v", got)
	}
	// Again: nothing to write.
	if err := cp.(processor.PayerProfiles).ProfileCustomer(context.Background(), str(old["id"]), profile); err != nil {
		t.Fatal(err)
	}
	_ = db
}
