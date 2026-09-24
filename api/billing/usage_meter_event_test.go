package billing

import (
	"fmt"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/meter"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/test/ae"
)

// seedUsageMeter creates the "api-usage" meter RecordUsage looks for, summing the
// quantity its events carry.
func seedUsageMeter(t *testing.T, db *datastore.Datastore) *meter.Meter {
	t.Helper()
	m := meter.New(db)
	m.Name = "API usage"
	m.EventName = "api-usage"
	m.AggregationType = meter.AggSum
	if err := m.Create(); err != nil {
		t.Fatalf("create meter: %v", err)
	}
	return m
}

// usageEvents polls for the meter events RecordUsage writes from a goroutine.
func usageEvents(t *testing.T, db *datastore.Datastore, meterID string, want int) []*meter.MeterEvent {
	t.Helper()
	rootKey := db.NewKey("synckey", "", 1, nil)
	deadline := time.Now().Add(3 * time.Second)
	for {
		evts := make([]*meter.MeterEvent, 0, want)
		if _, err := meter.QueryEvents(db).Ancestor(rootKey).Filter("MeterId=", meterID).GetAll(&evts); err == nil && len(evts) >= want {
			return evts
		}
		if time.Now().After(deadline) {
			t.Fatalf("meter events for %s: never reached %d", meterID, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// A METER EVENT'S VALUE IS A QUANTITY, AND THE CHARGE IS NOT ONE.
//
// AggregateUsage sums Value and hands the sum to PricingRule.CalculateCost, so a
// value that is already money is priced a second time — the invoice line charges
// for the charge. RecordUsage wrote req.Amount, the debit in cents, which was
// wrong twice over: non-zero it double-charged, and on the preferred amountMicros
// path (where `amount` is absent) it was zero, so the line silently vanished.
//
// Both readings are asserted here from the SAME request, because a fix that only
// stops the zero would leave the double-charge and look green.
func TestAMeterEventCarriesTheQuantityNotTheCharge(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := &organization.Organization{}
	org.Name = "usage-meter-org"
	org.Live = true
	db := datastore.New(org.Namespaced(ctx))
	m := seedUsageMeter(t, db)

	// The modern path: the charge crosses as micro-USD and `amount` is absent.
	// 1234 micros is $0.001234 — a real sub-cent charge that rounds to one cent.
	// 1234 micros is $0.001234 — under half a cent, so the CHARGE rounds to $0.00
	// and RecordUsage acknowledges it without a debit. The work still happened.
	if got := recordUsage(t, org, `{
		"user":"usage-meter-org/alice@example.com",
		"amountMicros":1234,
		"model":"gpt-5","provider":"hanzo",
		"promptTokens":120,"completionTokens":30,"totalTokens":150,
		"requestId":"req-quantity-1"
	}`); got != 200 {
		t.Fatalf("RecordUsage status = %d, want 200 (rounded-to-zero is acknowledged)", got)
	}

	evts := usageEvents(t, db, m.Id(), 1)
	if n := len(evts); n != 1 {
		t.Fatalf("meter events = %d, want exactly 1 per act", n)
	}
	switch v := evts[0].Value; v {
	case 150:
		// the tokens the charge priced — what a pricing rule can price
	case 0:
		t.Fatal("meter event value = 0: the value was read from `amount`, which the " +
			"amountMicros path never sends, so the usage line vanishes from the invoice")
	default:
		t.Fatalf("meter event value = %d, want 150 (the tokens). A value of 1 would be "+
			"the rounded cents — money, which AggregateUsage would price a second time", v)
	}
	if got := fmt.Sprint(evts[0].Dimensions["model"]); got != "gpt-5" {
		t.Fatalf("dimension model = %q, want gpt-5", got)
	}
}

// The cents path must reach the same answer. A caller still sending `amount` is
// not sending a quantity, and reading one out of it is the whole defect — so the
// tokens decide here too, and the cents are only money.
func TestTheCentsPathAlsoMetersTheQuantity(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := &organization.Organization{}
	org.Name = "usage-meter-cents-org"
	org.Live = true
	db := datastore.New(org.Namespaced(ctx))
	m := seedUsageMeter(t, db)

	if got := recordUsage(t, org, `{
		"user":"usage-meter-cents-org/bob@example.com",
		"amount":250,
		"model":"gpt-5","provider":"hanzo",
		"promptTokens":800,"completionTokens":200,"totalTokens":1000,
		"requestId":"req-quantity-2"
	}`); got != 201 {
		t.Fatalf("RecordUsage status = %d, want 201", got)
	}

	evts := usageEvents(t, db, m.Id(), 1)
	if v := evts[0].Value; v != 1000 {
		t.Fatalf("meter event value = %d, want 1000 (tokens). 250 is the charge in "+
			"cents, and pricing it again is a double charge", v)
	}
}

// A retry is the same act, and an act is metered once. The transaction has had an
// idempotency guard for a while; the meter event was a hand-rolled copy of
// engine.IngestUsageEvent WITHOUT its dedup, so every retry of a streamed
// completion counted its tokens again and inflated the invoice quantity.
func TestARetryMetersTheActOnce(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := &organization.Organization{}
	org.Name = "usage-meter-retry-org"
	org.Live = true
	db := datastore.New(org.Namespaced(ctx))
	m := seedUsageMeter(t, db)

	body := `{
		"user":"usage-meter-retry-org/carol@example.com",
		"amount":250,
		"model":"gpt-5","provider":"hanzo",
		"promptTokens":800,"completionTokens":200,"totalTokens":1000,
		"requestId":"req-retried-once"
	}`
	recordUsage(t, org, body)
	evts := usageEvents(t, db, m.Id(), 1)
	recordUsage(t, org, body) // the client lost the response and tried again

	// Give a second row every chance to appear before concluding there isn't one.
	time.Sleep(300 * time.Millisecond)
	after := usageEvents(t, db, m.Id(), 1)
	if len(after) != len(evts) {
		t.Fatalf("meter events after retry = %d, want %d — a retry counted the tokens twice",
			len(after), len(evts))
	}
	if v := after[0].Value; v != 1000 {
		t.Fatalf("metered quantity = %d, want 1000", v)
	}
}
