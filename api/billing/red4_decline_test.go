package billing

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment/processor"
	squarelib "github.com/hanzoai/commerce/thirdparty/square"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestRed4_APinLargerThanWhatIsOwedIsChargedInFull — the card is never asked for more
// than the invoice still owes, whatever amount is pinned: a pin of 5000 on a 1900
// invoice charges 1900. The pin is a typed field no endpoint binds, and request
// metadata naming one is inert.
func TestRed4_APinLargerThanWhatIsOwedIsChargedInFull(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red4-over")
	db := datastore.New(org.Namespaced(ctx))
	m := squareMock("", "", "sqpay_over")
	withFakeSquare(t, m)
	sub := seedCardBackedSub(t, db, "red4-over", "dev", "ccof_o", "cust_o")
	inv := seedOpenInvoice(t, db, sub, 1900)
	inv.Metadata = map[string]interface{}{"unresolvedCardCents": "9000", "unresolvedCents": 9000}
	inv.UnresolvedCents = 5000

	res, err := engine.CollectInvoice(ctx, db, inv, &movingPrepaid{}, chargeProviderForOrg(org))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("charged=%d amountDue=%d amountPaid=%d status=%s", m.lastChargeAmount, inv.AmountDue, inv.AmountPaid, inv.Status)
	if m.lastChargeAmount > inv.AmountDue || inv.AmountPaid != inv.AmountDue {
		t.Errorf("the card was charged %d for an invoice owing %d, which records %d paid (result %+v)", m.lastChargeAmount, inv.AmountDue, inv.AmountPaid, res)
	}
}

// disabledSquare is Square with key memory: a request that never reached it (the
// first, lost in transit) stored nothing, and the pinned card is disabled by the
// time it is resent, which Square answers outside PAYMENT_METHOD_ERROR.
type disabledSquare struct {
	mu      sync.Mutex
	calls   int
	sources []string
}

func (s *disabledSquare) RoundTrip(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, _ := io.ReadAll(r.Body)
	var req struct {
		Source string `json:"source_id"`
	}
	_ = json.Unmarshal(raw, &req)
	s.calls++
	s.sources = append(s.sources, req.Source)
	if s.calls == 1 {
		return nil, errors.New("read tcp: connection reset by peer")
	}
	if req.Source == "ccof_old" {
		return &http.Response{StatusCode: 404, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{"errors":[{"category":"INVALID_REQUEST_ERROR","code":"NOT_FOUND","detail":"Card not found."}]}`)), Request: r}, nil
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{"payment":{"id":"sqpay_new","status":"COMPLETED","amount_money":{"amount":1900,"currency":"USD"}}}`)), Request: r}, nil
}

// TestRed4_APinnedCardThatCanNeverAnswerWedgesTheInvoice — the first renewal attempt
// is lost in transit, so Square never stored its key and the card is pinned. The buyer
// replaces the card, and the resend of the pinned (now deleted) card is answered with
// a 4xx outside PAYMENT_METHOD_ERROR: no payment exists under the key, so that is an
// outcome, the pin is cleared, and the next attempt charges the buyer's new card.
func TestRed4_APinnedCardThatCanNeverAnswerWedgesTheInvoice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	sq := &disabledSquare{}
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

	org := moneyOrg("red4-disabled")
	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, "red4-disabled", "dev", "ccof_old", "cust_d")
	inv := seedOpenInvoice(t, db, sub, 1900)
	if _, err := engine.CollectInvoice(ctx, db, inv, &movingPrepaid{}, chargeProviderForOrg(org)); err != nil {
		t.Fatal(err)
	}
	// The buyer saves a new card and makes it the subscription's default.
	pm := seedSavedCard(t, db, "red4-disabled", "ccof_new", "cust_d")
	sub.DefaultPaymentMethod = pm.Id()
	if err := sub.Update(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3 && inv.Status != billinginvoice.Paid; i++ {
		if _, err := engine.CollectInvoice(ctx, db, inv, &movingPrepaid{}, chargeProviderForOrg(org)); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("sources=%v status=%s attempts=%d metadata=%v", sq.sources, inv.Status, inv.AttemptCount, inv.Metadata)
	if inv.Status != billinginvoice.Paid {
		t.Errorf("after %d attempts the invoice is %s: every attempt resent the pinned card %v and the buyer's new card was never tried",
			sq.calls, inv.Status, sq.sources[1:])
	}
}

// TestRed4_ThePinSurvivesAReload — production loads the invoice afresh for every
// attempt (PayInvoice's core, the renewal cycle), so the pin only works if it is
// persisted with the invoice.
func TestRed4_ThePinSurvivesAReload(t *testing.T) {
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

	org := moneyOrg("red4-reload")
	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, "red4-reload", "dev", "ccof_rl", "cust_rl")
	inv := seedOpenInvoice(t, db, sub, 1900)
	if _, err := engine.CollectInvoice(ctx, db, inv, &movingPrepaid{}, chargeProviderForOrg(org)); err != nil {
		t.Fatal(err)
	}
	if err := inv.Update(); err != nil {
		t.Fatal(err)
	}
	fresh := billinginvoice.New(db)
	if err := fresh.GetById(inv.Id()); err != nil {
		t.Fatal(err)
	}
	if fresh.UnresolvedCents != 1900 || fresh.UnresolvedCard != "ccof_rl" || fresh.UnresolvedCustomer != "cust_rl" {
		t.Errorf("the pin did not survive a reload: %d %q %q", fresh.UnresolvedCents, fresh.UnresolvedCard, fresh.UnresolvedCustomer)
	}
	sq.status = "COMPLETED"
	if _, err := engine.CollectInvoice(ctx, db, fresh, &movingPrepaid{avail: 100}, chargeProviderForOrg(org)); err != nil {
		t.Fatal(err)
	}
	if fresh.Status != billinginvoice.Paid {
		t.Errorf("the reloaded invoice answered %s after the resend (codes %v)", fresh.Status, sq.codes)
	}
}
