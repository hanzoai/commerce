package billing

// Saved cards must be CHARGEABLE and SINGULAR. These pin the three facts the
// card-on-file rework added:
//
//	subscribe + top-up accept paymentMethodId — the vaulted card is charged
//	  directly, nothing is re-entered, no new row is created
//	saving the same card twice yields ONE row (fingerprint dedupe), and the
//	  redundant Square card-on-file is detached again
//	a vaulted row carries the card's brand/last4/expiry, and rows saved before
//	  that heal from the processor's card list

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestSubscribeWithCard_SavedMethod charges an already-saved card by its
// paymentMethodId: no re-vault, no new row, the saved card-on-file id charged
// in its Square customer's context, and the method becomes the default.
func TestSubscribeWithCard_SavedMethod(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("savedsub")
	db := datastore.New(org.Namespaced(ctx))
	pm := seedSavedCard(t, db, "savedsub", "ccof_saved", "cust_saved")

	m := squareMock("cust_never", "ccof_never", "sqpay_saved")
	withFakeSquare(t, m)

	body := fmt.Sprintf(`{"paymentMethodId":%q,"planId":"pro"}`, pm.Id())
	resp := invokeSubscribeCard(org, ctx, body, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d, want 201", resp.StatusCode)
	}
	out := jsonBody(t, resp)
	if out["paymentMethodId"] != pm.Id() {
		t.Fatalf("response paymentMethodId=%v, want %q", out["paymentMethodId"], pm.Id())
	}

	// The SAVED card was charged — and nothing was vaulted.
	if m.chargeCalls != 1 {
		t.Fatalf("charge calls=%d, want 1", m.chargeCalls)
	}
	if m.lastChargeToken != "ccof_saved" || m.lastChargeCustomer != "cust_saved" {
		t.Fatalf("charged %q for customer %q, want ccof_saved/cust_saved", m.lastChargeToken, m.lastChargeCustomer)
	}
	if m.createCustomerCalls != 0 || m.addCardNonce != "" {
		t.Fatal("a saved-method subscribe must not vault anything")
	}

	// Still exactly one row, now the default, and the subscription points at it.
	pms := pmsFor(t, db, "savedsub")
	if len(pms) != 1 {
		t.Fatalf("payment methods=%d, want 1 (no new row for a saved-method charge)", len(pms))
	}
	if !pms[0].IsDefault {
		t.Fatal("the charged saved method should be default")
	}
	sub := parentSub(t, db, "savedsub", "pro")
	if sub == nil || sub.DefaultPaymentMethod != pm.Id() {
		t.Fatalf("subscription DefaultPaymentMethod=%v, want %q", sub, pm.Id())
	}
}

// TestSubscribeWithCard_SavedMethodOtherSubject: naming another subject's saved
// method is 404 (no existence oracle) and charges nothing.
func TestSubscribeWithCard_SavedMethodOtherSubject(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("savedidor")
	db := datastore.New(org.Namespaced(ctx))
	other := seedSavedCard(t, db, "savedidor/alice", "ccof_alice", "cust_alice")

	m := squareMock("c", "cc", "ref")
	withFakeSquare(t, m)

	body := fmt.Sprintf(`{"paymentMethodId":%q,"planId":"pro"}`, other.Id())
	resp := invokeSubscribeCard(org, ctx, body, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, want 404 for another subject's method", resp.StatusCode)
	}
	if m.chargeCalls != 0 {
		t.Fatal("another subject's card must never be charged")
	}
}

// TestSubscribeWithCard_ExactlyOneCardInput: neither or both of sourceId /
// paymentMethodId is a 400 before any money moves.
func TestSubscribeWithCard_ExactlyOneCardInput(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cardinput")
	m := squareMock("c", "cc", "ref")
	withFakeSquare(t, m)

	for _, body := range []string{
		`{"planId":"pro"}`,
		`{"sourceId":"cnon:x","paymentMethodId":"pm_x","planId":"pro"}`,
	} {
		resp := invokeSubscribeCard(org, ctx, body, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("body %s → status=%d, want 400", body, resp.StatusCode)
		}
	}
	if m.chargeCalls != 0 || m.createCustomerCalls != 0 {
		t.Fatal("invalid card input must not reach the processor")
	}
}

// TestSaveCard_StampsFacts: a vaulted row records what the processor reported —
// brand/last4/expiry on the row, the receipt name, the fingerprint in metadata.
func TestSaveCard_StampsFacts(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("stamp")
	db := datastore.New(org.Namespaced(ctx))

	m := squareMock("cust_f", "ccof_f", "ref")
	m.vaultCard = processor.Card{Brand: "Visa", Last4: "4242", ExpMonth: 12, ExpYear: 2030, Fingerprint: "fp-visa-4242"}
	withFakeSquare(t, m)

	pm, created, err := saveCard(context.Background(), db, m, "stamp", "buyer@acme.test", "cnon:ok")
	if err != nil || !created {
		t.Fatalf("saveCard: created=%v err=%v", created, err)
	}
	if pm.Card == nil || pm.Card.Brand != "Visa" || pm.Card.Last4 != "4242" || pm.Card.ExpYear != 2030 {
		t.Fatalf("card facts not stamped: %+v", pm.Card)
	}
	if pm.Name != "Visa ending in 4242" {
		t.Fatalf("pm.Name=%q, want the receipt name", pm.Name)
	}
	if metaStr(pm, "fingerprint") != "fp-visa-4242" {
		t.Fatalf("fingerprint metadata=%q, want fp-visa-4242", metaStr(pm, "fingerprint"))
	}
}

// TestSaveCard_DedupesByFingerprint: saving the same card again returns the
// EXISTING row and detaches the redundant Square card-on-file — one card, one
// row, structurally.
func TestSaveCard_DedupesByFingerprint(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("dedupe")
	db := datastore.New(org.Namespaced(ctx))

	m := squareMock("cust_d", "ccof_first", "ref")
	m.vaultCard = processor.Card{Brand: "Visa", Last4: "4242", ExpMonth: 12, ExpYear: 2030, Fingerprint: "fp-dup"}
	withFakeSquare(t, m)

	first, created, err := saveCard(context.Background(), db, m, "dedupe", "", "cnon:one")
	if err != nil || !created {
		t.Fatalf("first save: created=%v err=%v", created, err)
	}

	// Same card, new nonce, new Square card id.
	m.addedCardID = "ccof_second"
	m.vaultCard.ID = "ccof_second"
	second, created, err := saveCard(context.Background(), db, m, "dedupe", "", "cnon:two")
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if created {
		t.Fatal("saving the same card twice must not create a second row")
	}
	if second.Id() != first.Id() {
		t.Fatalf("second save returned row %s, want the existing %s", second.Id(), first.Id())
	}
	if !m.removeCalled || m.removeCardID != "ccof_second" {
		t.Fatalf("the redundant Square card must be detached (removeCalled=%v id=%q)", m.removeCalled, m.removeCardID)
	}
	if rows := pmsFor(t, db, "dedupe"); len(rows) != 1 {
		t.Fatalf("rows=%d, want 1", len(rows))
	}
}

// TestHeal_BackfillsCardFacts: rows vaulted before facts were recorded gain
// brand/last4/fingerprint from ONE processor list call, persisted.
func TestHeal_BackfillsCardFacts(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("healorg")
	db := datastore.New(org.Namespaced(ctx))

	a := seedSavedCard(t, db, "healorg", "ccof_a", "cust_h")
	b := seedSavedCard(t, db, "healorg", "ccof_b", "cust_h")

	m := squareMock("cust_h", "x", "ref")
	m.listCards = []processor.Card{
		{ID: "ccof_a", Brand: "Visa", Last4: "4242", ExpMonth: 12, ExpYear: 2030, Fingerprint: "fp-a"},
		{ID: "ccof_b", Brand: "Mastercard", Last4: "4444", ExpMonth: 3, ExpYear: 2031, Fingerprint: "fp-b"},
	}
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(m)

	heal(context.Background(), reg, []*paymentmethod.PaymentMethod{a, b})

	// Persisted, not just in-memory: reload both rows.
	for id, want := range map[string]string{a.Id(): "Visa ending in 4242", b.Id(): "Mastercard ending in 4444"} {
		pm := paymentmethod.New(db)
		if err := pm.GetById(id); err != nil {
			t.Fatalf("reload %s: %v", id, err)
		}
		if pm.Card == nil || pm.Name != want {
			t.Fatalf("row %s not healed: name=%q card=%+v, want %q", id, pm.Name, pm.Card, want)
		}
		if metaStr(pm, "fingerprint") == "" {
			t.Fatalf("row %s missing fingerprint after heal", id)
		}
	}
}

// TestTopup_SavedMethodOtherSubject: POST /v1/billing/topup naming another
// subject's method is 404 (no existence oracle) and charges nothing — charging
// someone else's card to fund your own balance is the exact cross-subject move
// the guard refuses.
func TestTopup_SavedMethodOtherSubject(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("topupidor")
	db := datastore.New(org.Namespaced(ctx))
	other := seedSavedCard(t, db, "topupidor/alice", "ccof_al", "cust_al")

	m := squareMock("c", "cc", "ref")
	withFakeSquare(t, m)

	body := fmt.Sprintf(`{"paymentMethodId":%q,"amountCents":2500}`, other.Id())
	resp := invokeTopup(org, ctx, body, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", resp.StatusCode)
	}
	if m.chargeCalls != 0 {
		t.Fatal("another subject's card must never be charged")
	}
}
