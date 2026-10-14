package billing

// A billing subject has one Square customer, found or made once:
//
//	a declined card and a retry make one customer, not two
//	saves at once for one subject make one customer
//	a subject with a saved card reuses its customer and asks Square nothing
//	a failed search still makes the customer, under its key

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/test/ae"
)

// slowCreate holds each customer create open for a moment, as Square's round
// trip does, so saves that nothing orders all search before any create lands.
type slowCreate struct{ *MockSquareProcessor }

func (s slowCreate) CreateCustomer(ctx context.Context, email, name string, meta map[string]interface{}) (string, error) {
	time.Sleep(20 * time.Millisecond)
	return s.MockSquareProcessor.CreateCustomer(ctx, email, name, meta)
}

func TestSaveCard_ADeclineAndARetryMakeOneCustomer(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("custretry")
	db := datastore.New(org.Namespaced(ctx))

	m := squareMock("cust_once", "ccof_once", "ref")
	m.addCardErr = errors.New("CARD_DECLINED")
	if _, _, err := saveCard(context.Background(), db, m, "custretry", "buyer@acme.test", "cnon:bad"); err == nil {
		t.Fatal("a declined card must fail the save")
	}
	if m.createCustomerCalls != 1 || m.customers["custretry/custretry"] != "cust_once" {
		t.Fatalf("first save: creates=%d customers=%v, want one customer under custretry/custretry",
			m.createCustomerCalls, m.customers)
	}

	m.addCardErr = nil
	pm, created, err := saveCard(context.Background(), db, m, "custretry", "buyer@acme.test", "cnon:good")
	if err != nil || !created {
		t.Fatalf("retry: created=%v err=%v", created, err)
	}
	if m.createCustomerCalls != 1 {
		t.Fatalf("CreateCustomer calls=%d after a decline and a retry, want 1", m.createCustomerCalls)
	}
	if got := metaStr(pm, "squareCustomerId"); got != "cust_once" {
		t.Fatalf("saved card names customer %q, want cust_once", got)
	}
}

func TestSaveCard_SavesAtOnceMakeOneCustomer(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("custrace")

	m := squareMock("cust_race", "ccof_race", "ref")
	m.addCardErr = errors.New("CARD_DECLINED")
	cp := slowCreate{m}

	const n = 8
	var wg sync.WaitGroup
	declined := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			own := datastore.New(org.Namespaced(ctx))
			_, _, err := saveCard(context.Background(), own, cp, "custrace", "buyer@acme.test", "cnon:bad")
			declined <- err
		}()
	}
	wg.Wait()
	close(declined)
	for err := range declined {
		if err == nil {
			t.Fatal("a declined card must fail every save")
		}
	}
	if m.createCustomerCalls != 1 {
		t.Fatalf("CreateCustomer calls=%d for %d saves at once, want 1", m.createCustomerCalls, n)
	}
}

func TestSaveCard_ASavedCardsCustomerIsReused(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("custsaved")
	db := datastore.New(org.Namespaced(ctx))
	seedSavedCard(t, db, "custsaved", "ccof_old", "cust_saved")

	m := squareMock("cust_never", "ccof_new", "ref")
	if _, _, err := saveCard(context.Background(), db, m, "custsaved", "buyer@acme.test", "cnon:new"); err != nil {
		t.Fatalf("saveCard: %v", err)
	}
	if m.findCalls != 0 || m.createCustomerCalls != 0 {
		t.Fatalf("searches=%d creates=%d, want neither for a subject with a saved card", m.findCalls, m.createCustomerCalls)
	}
	if m.addCardCustomerID != "cust_saved" {
		t.Fatalf("vaulted under %q, want the saved card's customer cust_saved", m.addCardCustomerID)
	}
}

func TestSaveCard_AFailedSearchStillMakesTheCustomer(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("custsearch")
	db := datastore.New(org.Namespaced(ctx))

	m := squareMock("cust_s", "ccof_s", "ref")
	m.findErr = errors.New("503 Service Unavailable")
	pm, _, err := saveCard(context.Background(), db, m, "custsearch", "buyer@acme.test", "cnon:ok")
	if err != nil {
		t.Fatalf("saveCard: %v", err)
	}
	if m.createCustomerCalls != 1 || metaStr(pm, "squareCustomerId") != "cust_s" {
		t.Fatalf("creates=%d customer=%q, want the customer made once", m.createCustomerCalls, metaStr(pm, "squareCustomerId"))
	}
}
