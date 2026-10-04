package billing

// A subject has at most one default card, and auto-recharge charges it. Saving
// decides it for the first card:
//
//	the first card a subject saves is its default; a later save leaves it alone
//	a save the processor refuses changes nothing
//	two saves at once leave exactly one default

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// vaultSeq vaults every nonce as a card of its own — a fresh id and fingerprint
// per call — and may be called from many goroutines at once. With gate set, each
// vault waits until gate's count of callers is inside, so their saves reach the
// default decision together.
type vaultSeq struct {
	*MockSquareProcessor
	n    atomic.Int64
	gate *sync.WaitGroup
}

func (v *vaultSeq) CreateCustomer(context.Context, string, string, map[string]interface{}) (string, error) {
	return "cust_seq", nil
}

func (v *vaultSeq) Vault(_ context.Context, customerID, _ string) (processor.Card, error) {
	i := v.n.Add(1)
	if v.gate != nil {
		v.gate.Done()
		v.gate.Wait()
	}
	return processor.Card{
		ID: fmt.Sprintf("ccof_%d", i), CustomerID: customerID,
		Brand: "Visa", Last4: fmt.Sprintf("%04d", i), ExpMonth: 12, ExpYear: 2030,
		Fingerprint: fmt.Sprintf("fp-%d", i),
	}, nil
}

// card persists a vaulted card row for subject, default or not.
func card(t *testing.T, db *datastore.Datastore, subject, ref string, def bool) *paymentmethod.PaymentMethod {
	t.Helper()
	pm := paymentmethod.New(db)
	pm.CustomerId = subject
	pm.UserId = subject
	pm.Type = "card"
	pm.ProviderRef = ref
	pm.ProviderType = string(processor.Square)
	pm.IsDefault = def
	pm.Metadata = map[string]interface{}{"squareCustomerId": "cust_" + subject, "squareCardId": ref}
	if err := pm.Create(); err != nil {
		t.Fatalf("seed card: %v", err)
	}
	return pm
}

// defaults names the subject's default cards, as auto-recharge would find them.
func defaults(t *testing.T, db *datastore.Datastore, subject string) []string {
	t.Helper()
	out := []string{}
	for _, pm := range pmsFor(t, db, subject) {
		if pm.IsDefault {
			out = append(out, pm.Id())
		}
	}
	return out
}

func TestSaveCard_TheFirstCardIsTheDefault(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("deffirst")
	db := datastore.New(org.Namespaced(ctx))
	v := &vaultSeq{MockSquareProcessor: squareMock("cust_seq", "", "ref")}

	pm, created, err := saveCard(context.Background(), db, v, "deffirst", "", "cnon:one")
	if err != nil || !created {
		t.Fatalf("saveCard: created=%v err=%v", created, err)
	}
	if !pm.IsDefault {
		t.Fatal("the first card a subject saves must be its default")
	}
	if got := defaults(t, db, "deffirst"); len(got) != 1 || got[0] != pm.Id() {
		t.Fatalf("stored defaults=%v, want [%s]", got, pm.Id())
	}
	if def := defaultPaymentMethod(db, "deffirst"); def == nil || def.Id() != pm.Id() {
		t.Fatalf("auto-recharge reads default %v, want %s", def, pm.Id())
	}
}

func TestSaveCard_ALaterCardLeavesTheDefault(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("deflater")
	db := datastore.New(org.Namespaced(ctx))
	first := card(t, db, "deflater", "ccof_first", true)
	v := &vaultSeq{MockSquareProcessor: squareMock("cust_seq", "", "ref")}

	pm, created, err := saveCard(context.Background(), db, v, "deflater", "", "cnon:two")
	if err != nil || !created {
		t.Fatalf("saveCard: created=%v err=%v", created, err)
	}
	if pm.IsDefault {
		t.Fatal("a card saved while the subject has a default must not become the default")
	}
	if got := defaults(t, db, "deflater"); len(got) != 1 || got[0] != first.Id() {
		t.Fatalf("defaults=%v, want only the first card %s", got, first.Id())
	}
}

func TestSaveCard_AnOrgWithCardsAndNoDefaultTakesTheNewCard(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("defnone")
	db := datastore.New(org.Namespaced(ctx))
	old := card(t, db, "defnone", "ccof_old", false)
	v := &vaultSeq{MockSquareProcessor: squareMock("cust_seq", "", "ref")}

	pm, _, err := saveCard(context.Background(), db, v, "defnone", "", "cnon:new")
	if err != nil {
		t.Fatalf("saveCard: %v", err)
	}
	if !pm.IsDefault {
		t.Fatal("a card saved while the subject has no default must become the default")
	}
	got := map[string]bool{}
	for _, row := range pmsFor(t, db, "defnone") {
		got[row.Id()] = row.IsDefault
	}
	if got[old.Id()] {
		t.Fatal("the card already on file must stay as it was")
	}
}

func TestSaveCard_ARefusedSaveChangesNoDefault(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("defrefused")
	db := datastore.New(org.Namespaced(ctx))

	m := squareMock("cust_r", "ccof_r", "ref")
	m.addCardErr = errors.New("CARD_DECLINED")
	if _, _, err := saveCard(context.Background(), db, m, "defrefused", "", "cnon:bad"); err == nil {
		t.Fatal("a refused vault must fail the save")
	}
	if rows := pmsFor(t, db, "defrefused"); len(rows) != 0 {
		t.Fatalf("rows=%d after a refused save into an empty org, want 0", len(rows))
	}

	held := card(t, db, "defrefused", "ccof_held", true)
	if _, _, err := saveCard(context.Background(), db, m, "defrefused", "", "cnon:bad"); err == nil {
		t.Fatal("a refused vault must fail the save")
	}
	if got := defaults(t, db, "defrefused"); len(got) != 1 || got[0] != held.Id() {
		t.Fatalf("defaults=%v after a refused save, want [%s]", got, held.Id())
	}
}

func TestSaveCard_TwoSavesAtOnceLeaveOneDefault(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("defrace")
	db := datastore.New(org.Namespaced(ctx))

	const n = 8
	gate := &sync.WaitGroup{}
	gate.Add(n)
	v := &vaultSeq{MockSquareProcessor: squareMock("cust_seq", "", "ref"), gate: gate}

	// One datastore per save, as one request each holds its own.
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			own := datastore.New(org.Namespaced(ctx))
			if _, _, err := saveCard(context.Background(), own, v, "defrace", "", fmt.Sprintf("cnon:%d", i)); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("saveCard: %v", err)
	}
	if rows := pmsFor(t, db, "defrace"); len(rows) != n {
		t.Fatalf("rows=%d, want %d", len(rows), n)
	}
	if got := defaults(t, db, "defrace"); len(got) != 1 {
		t.Fatalf("defaults=%v after %d concurrent first saves, want exactly one", got, n)
	}
}
