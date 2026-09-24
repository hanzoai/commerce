package billing

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment/processor"
	squarelib "github.com/hanzoai/commerce/thirdparty/square"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestRenewal_AnUnresolvedAttemptResendsItsCard — Square answers a key only for the
// request it first saw, and the card is part of that request. A renewal whose outcome
// Square has not stated is resent with the card it was sent with, even when the
// subscription's default card changed in between, and the invoice row carries that
// card across a reload. A settled answer clears it.
func TestRenewal_AnUnresolvedAttemptResendsItsCard(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	sq := &idemSquare{status: "PENDING"}
	old := http.DefaultClient.Transport
	http.DefaultClient.Transport = sq
	t.Cleanup(func() { http.DefaultClient.Transport = old })
	sp := squarelib.NewProcessor(squarelib.Config{AccessToken: "sq-test", LocationID: "L1", Environment: "sandbox"})
	prev := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(sp)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = prev })

	org := moneyOrg("renew-pin")
	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, "renew-pin", "dev", "ccof_first", "cust_first")
	inv := seedOpenInvoice(t, db, sub, 1900)

	if _, err := engine.CollectInvoice(ctx, db, inv, nil, chargeProviderForOrg(org)); err != nil {
		t.Fatal(err)
	}
	if err := inv.Update(); err != nil {
		t.Fatal(err)
	}
	// The buyer swaps the subscription's card while the payment settles.
	next := seedSavedCard(t, db, "renew-pin", "ccof_next", "cust_next")
	sub.DefaultPaymentMethod = next.Id()
	if err := sub.Update(); err != nil {
		t.Fatal(err)
	}
	sq.status = "COMPLETED"

	again := billinginvoice.New(db)
	if err := again.GetById(inv.Id()); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.CollectInvoice(ctx, db, again, nil, chargeProviderForOrg(org)); err != nil {
		t.Fatal(err)
	}
	if again.Status != billinginvoice.Paid || fmt.Sprint(sq.codes) != "[200 200]" || sq.keys[0] != sq.keys[1] {
		t.Fatalf("the resent renewal: invoice %s, square answered %v under keys %v", again.Status, sq.codes, sq.keys)
	}
	if again.UnresolvedCard != "" || again.UnresolvedCustomer != "" || again.UnresolvedCents != 0 {
		t.Errorf("the pin outlived a settled answer: %q %q %d", again.UnresolvedCard, again.UnresolvedCustomer, again.UnresolvedCents)
	}
}

// TestRenewal_ARefusedAttemptPinsNothing — a refusal is a definite answer: the next
// attempt goes under a key of its own, so it charges the subscription's card as it is
// then, not the card that was refused.
func TestRenewal_ARefusedAttemptPinsNothing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	sq := &idemSquare{status: "FAILED"}
	old := http.DefaultClient.Transport
	http.DefaultClient.Transport = sq
	t.Cleanup(func() { http.DefaultClient.Transport = old })
	sp := squarelib.NewProcessor(squarelib.Config{AccessToken: "sq-test", LocationID: "L1", Environment: "sandbox"})
	prev := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(sp)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = prev })

	org := moneyOrg("renew-refused")
	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, "renew-refused", "dev", "ccof_refused", "cust_refused")
	inv := seedOpenInvoice(t, db, sub, 1900)
	if _, err := engine.CollectInvoice(ctx, db, inv, nil, chargeProviderForOrg(org)); err != nil {
		t.Fatal(err)
	}
	if inv.UnresolvedCard != "" || inv.AttemptCount != 1 {
		t.Fatalf("a refused renewal left pin %q at attempt %d", inv.UnresolvedCard, inv.AttemptCount)
	}
	next := seedSavedCard(t, db, "renew-refused", "ccof_next", "cust_next")
	sub.DefaultPaymentMethod = next.Id()
	if err := sub.Update(); err != nil {
		t.Fatal(err)
	}
	sq.status = "COMPLETED"
	if _, err := engine.CollectInvoice(ctx, db, inv, nil, chargeProviderForOrg(org)); err != nil {
		t.Fatal(err)
	}
	if inv.Status != billinginvoice.Paid || sq.keys[0] == sq.keys[1] || sq.sources[1] != "ccof_next" {
		t.Errorf("after a refusal the renewal: invoice %s, keys %v, cards %v", inv.Status, sq.keys, sq.sources)
	}
}

// TestRenewal_ARefusedResendReleasesTheCard — a renewal still processing pins its
// card; when the resend under the same key is answered with a payment that failed,
// that is the outcome, and the next attempt goes under a key of its own with the
// subscription's card as it is then.
func TestRenewal_ARefusedResendReleasesTheCard(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	sq := &idemSquare{status: "PENDING"}
	old := http.DefaultClient.Transport
	http.DefaultClient.Transport = sq
	t.Cleanup(func() { http.DefaultClient.Transport = old })
	sp := squarelib.NewProcessor(squarelib.Config{AccessToken: "sq-test", LocationID: "L1", Environment: "sandbox"})
	prev := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(sp)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = prev })

	org := moneyOrg("renew-released")
	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, "renew-released", "dev", "ccof_held", "cust_held")
	inv := seedOpenInvoice(t, db, sub, 1900)
	collect := func() {
		t.Helper()
		if _, err := engine.CollectInvoice(ctx, db, inv, nil, chargeProviderForOrg(org)); err != nil {
			t.Fatal(err)
		}
	}
	collect()
	if inv.UnresolvedCard != "ccof_held" {
		t.Fatalf("a pending renewal pinned %q", inv.UnresolvedCard)
	}
	sq.status = "FAILED"
	collect()
	if inv.UnresolvedCard != "" || inv.UnresolvedCents != 0 || inv.AttemptCount != 1 {
		t.Fatalf("a failed resend left pin %q %d at attempt %d", inv.UnresolvedCard, inv.UnresolvedCents, inv.AttemptCount)
	}
	next := seedSavedCard(t, db, "renew-released", "ccof_after", "cust_after")
	sub.DefaultPaymentMethod = next.Id()
	if err := sub.Update(); err != nil {
		t.Fatal(err)
	}
	sq.status = "COMPLETED"
	collect()
	if inv.Status != billinginvoice.Paid || len(sq.sources) != 3 || sq.sources[2] != "ccof_after" || sq.keys[2] == sq.keys[0] {
		t.Errorf("after a failed resend the renewal: invoice %s, cards %v, keys %v", inv.Status, sq.sources, sq.keys)
	}
}

// TestRenewal_RequestMetadataPinsNothing — an invoice's metadata is whatever its
// creator sent, so nothing in it can name the amount or the card a renewal charges.
func TestRenewal_RequestMetadataPinsNothing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("renew-meta")
	db := datastore.New(org.Namespaced(ctx))
	m := squareMock("cust_meta", "ccof_meta", "sqpay_meta")
	withFakeSquare(t, m)
	sub := seedCardBackedSub(t, db, "renew-meta", "dev", "ccof_meta", "cust_meta")
	inv := seedOpenInvoice(t, db, sub, 1900)
	inv.Metadata = map[string]interface{}{
		"unresolvedCardCents": "5000", "unresolvedCents": 5000,
		"unresolvedCard": "ccof_other", "unresolvedCustomer": "cust_other",
	}
	if err := inv.Update(); err != nil {
		t.Fatal(err)
	}
	fresh := billinginvoice.New(db)
	if err := fresh.GetById(inv.Id()); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.CollectInvoice(ctx, db, fresh, nil, chargeProviderForOrg(org)); err != nil {
		t.Fatal(err)
	}
	if m.lastChargeAmount != 1900 || m.lastChargeToken != "ccof_meta" || m.lastChargeCustomer != "cust_meta" || fresh.Status != billinginvoice.Paid {
		t.Errorf("metadata steered the renewal: charged %d to %s/%s, invoice %s", m.lastChargeAmount, m.lastChargeToken, m.lastChargeCustomer, fresh.Status)
	}
}
