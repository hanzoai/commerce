package billing

// An explicit choice moves a subject's default to one of its own chargeable
// cards and to nothing else.

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/util/test/ae"
)

func TestSetDefaultMethod_MovesTheDefault(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("defmove")
	db := datastore.New(org.Namespaced(ctx))
	a := card(t, db, "defmove", "ccof_a", true)
	b := card(t, db, "defmove", "ccof_b", false)

	ch, err := SetDefaultMethod(ctx, org, b.Id(), "defmove")
	if err != nil {
		t.Fatalf("SetDefaultMethod: %v", err)
	}
	if ch.Method.Id != b.Id() || !ch.Method.IsDefault {
		t.Fatalf("answered %+v, want %s as the default", ch.Method, b.Id())
	}
	if len(ch.Previous) != 1 || ch.Previous[0] != a.Id() {
		t.Fatalf("previous=%v, want [%s]", ch.Previous, a.Id())
	}
	if got := defaults(t, db, "defmove"); len(got) != 1 || got[0] != b.Id() {
		t.Fatalf("defaults=%v, want [%s]", got, b.Id())
	}
	if def := defaultPaymentMethod(db, "defmove"); def == nil || def.Id() != b.Id() {
		t.Fatalf("auto-recharge reads default %v, want %s", def, b.Id())
	}
}

func TestSetDefaultMethod_TheDefaultAgainChangesNothing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("defsame")
	db := datastore.New(org.Namespaced(ctx))
	a := card(t, db, "defsame", "ccof_a", true)
	card(t, db, "defsame", "ccof_b", false)

	ch, err := SetDefaultMethod(ctx, org, a.Id(), "defsame")
	if err != nil {
		t.Fatalf("SetDefaultMethod: %v", err)
	}
	if len(ch.Previous) != 1 || ch.Previous[0] != a.Id() {
		t.Fatalf("previous=%v, want [%s]: the card was already the default", ch.Previous, a.Id())
	}
	if got := defaults(t, db, "defsame"); len(got) != 1 || got[0] != a.Id() {
		t.Fatalf("defaults=%v, want [%s]", got, a.Id())
	}
}

func TestSetDefaultMethod_ACardOfAnotherOrgIsNotFound(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	orgA, orgB := moneyOrg("defowna"), moneyOrg("defownb")
	dbA, dbB := datastore.New(orgA.Namespaced(ctx)), datastore.New(orgB.Namespaced(ctx))
	a := card(t, dbA, "defowna", "ccof_a", true)
	card(t, dbA, "defowna", "ccof_a2", false)
	b := card(t, dbB, "defownb", "ccof_b", false)

	if _, err := SetDefaultMethod(ctx, orgA, b.Id(), "defowna"); !IsMethodNotFound(err) {
		t.Fatalf("org A naming org B's card answered %v, want not found", err)
	}
	if got := defaults(t, dbA, "defowna"); len(got) != 1 || got[0] != a.Id() {
		t.Fatalf("org A defaults=%v, want [%s] unchanged", got, a.Id())
	}
	if got := defaults(t, dbB, "defownb"); len(got) != 0 {
		t.Fatalf("org B defaults=%v, want none: org A must not touch org B's cards", got)
	}
}

func TestSetDefaultMethod_ACardOfAnotherSubjectIsNotFound(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("defsubj")
	db := datastore.New(org.Namespaced(ctx))
	own := card(t, db, "defsubj", "ccof_own", true)
	theirs := card(t, db, "defsubj/alice", "ccof_alice", false)

	if _, err := SetDefaultMethod(ctx, org, theirs.Id(), "defsubj"); !IsMethodNotFound(err) {
		t.Fatalf("naming another subject's card answered %v, want not found", err)
	}
	if _, err := SetDefaultMethod(ctx, org, own.Id(), ""); !IsMethodNotFound(err) {
		t.Fatalf("a caller with no subject answered %v, want not found", err)
	}
	if _, err := SetDefaultMethod(ctx, org, "pm_nothing", "defsubj"); !IsMethodNotFound(err) {
		t.Fatalf("an id naming nothing answered %v, want not found", err)
	}
	if got := defaults(t, db, "defsubj"); len(got) != 1 || got[0] != own.Id() {
		t.Fatalf("defaults=%v, want [%s] unchanged", got, own.Id())
	}
	if got := defaults(t, db, "defsubj/alice"); len(got) != 0 {
		t.Fatalf("alice's defaults=%v, want none", got)
	}
}

func TestSetDefaultMethod_OnlyAChargeableCard(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("defwire")
	db := datastore.New(org.Namespaced(ctx))
	own := card(t, db, "defwire", "ccof_own", true)
	wire := paymentmethod.New(db)
	wire.CustomerId, wire.UserId, wire.Type = "defwire", "defwire", "wire"
	wire.Wire = &paymentmethod.WireDetails{BankName: "First"}
	if err := wire.Create(); err != nil {
		t.Fatalf("seed wire: %v", err)
	}
	bare := card(t, db, "defwire", "", false)

	for _, id := range []string{wire.Id(), bare.Id()} {
		if _, err := SetDefaultMethod(ctx, org, id, "defwire"); !IsMethodUnchargeable(err) {
			t.Fatalf("method %s answered %v, want unchargeable", id, err)
		}
	}
	if got := defaults(t, db, "defwire"); len(got) != 1 || got[0] != own.Id() {
		t.Fatalf("defaults=%v, want [%s] unchanged", got, own.Id())
	}
}

func TestSetDefaultMethod_ChoicesAtOnceLeaveOneDefault(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("defchoice")
	db := datastore.New(org.Namespaced(ctx))
	ids := []string{
		card(t, db, "defchoice", "ccof_1", true).Id(),
		card(t, db, "defchoice", "ccof_2", false).Id(),
		card(t, db, "defchoice", "ccof_3", false).Id(),
	}

	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if _, err := SetDefaultMethod(ctx, org, id, "defchoice"); err != nil {
				t.Errorf("SetDefaultMethod: %v", err)
			}
		}(ids[i%len(ids)])
	}
	wg.Wait()
	if got := defaults(t, db, "defchoice"); len(got) != 1 {
		t.Fatalf("defaults=%v after concurrent choices, want exactly one", got)
	}
}

// The commerce route names the customer in its path; a payment method id that is
// not that customer's must leave the customer's default where it was.
func TestSetDefaultPaymentMethod_AnUnknownIdKeepsTheDefault(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("defroute")
	db := datastore.New(org.Namespaced(ctx))
	held := card(t, db, "defroute", "ccof_held", true)

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/customers/defroute/default-payment-method",
		bytes.NewBufferString(`{"paymentMethodId":"pm_nothing"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := driveSeeded(func(c *zip.Ctx) {
		c.Locals("organization", org)
		c.SetContext(ctx)
	}, "/v1/billing/customers/:id/default-payment-method", req, SetDefaultPaymentMethod)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", resp.StatusCode)
	}
	if got := defaults(t, db, "defroute"); len(got) != 1 || got[0] != held.Id() {
		t.Fatalf("defaults=%v after a refused choice, want [%s] unchanged", got, held.Id())
	}
}

// A choice made at the commerce route moves the default and leaves both cards
// listed where every reader finds them.
func TestSetDefaultPaymentMethod_MovesTheDefaultAndKeepsTheCardsListed(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("defroutemove")
	db := datastore.New(org.Namespaced(ctx))
	a := card(t, db, "defroutemove", "ccof_a", true)
	b := card(t, db, "defroutemove", "ccof_b", false)

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/customers/defroutemove/default-payment-method",
		bytes.NewBufferString(`{"paymentMethodId":"`+b.Id()+`"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := driveSeeded(func(c *zip.Ctx) {
		c.Locals("organization", org)
		c.SetContext(ctx)
	}, "/v1/billing/customers/:id/default-payment-method", req, SetDefaultPaymentMethod)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want 200", resp.StatusCode)
	}
	if rows := pmsFor(t, db, "defroutemove"); len(rows) != 2 {
		t.Fatalf("listed cards=%d after the choice, want both", len(rows))
	}
	if got := defaults(t, db, "defroutemove"); len(got) != 1 || got[0] != b.Id() {
		t.Fatalf("defaults=%v, want [%s] (was %s)", got, b.Id(), a.Id())
	}
}
