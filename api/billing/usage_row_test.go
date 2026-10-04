package billing

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/meter"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// Every served call writes a usage row, whatever it costs.

func usageRows(t *testing.T, org *organization.Organization, subject string) []*transaction.Transaction {
	t.Helper()
	d := datastore.New(nscontext.WithNamespace(context.Background(), org.Name))
	rows := make([]*transaction.Transaction, 0)
	if _, err := transaction.Query(d).Ancestor(d.NewKey("synckey", "", 1, nil)).
		Filter("SourceId=", subject).Filter("Tags=", "api-usage").GetAll(&rows); err != nil {
		t.Fatalf("usage rows: %v", err)
	}
	return rows
}

func centsOfRows(rows []*transaction.Transaction) int64 {
	var n int64
	for _, r := range rows {
		n += int64(r.Amount)
	}
	return n
}

func metaInt(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	}
	return -1
}

// A $0 call is a row: no debit, and everything about the call kept — the model,
// the tokens, the scope, what it cost us and what paid.
func TestAZeroDollarCallWritesARow(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("usage-zero")
	const who = "usage-zero/alice"
	if got := recordUsage(t, org, `{"user":"`+who+`","amount":0,"costMicros":1500,"paidBy":"hanzo",
		"model":"zen5","provider":"hanzo","project":"p1","service":"chat",
		"promptTokens":10,"completionTokens":5,"totalTokens":15,"requestId":"zero-1"}`); got != http.StatusCreated {
		t.Fatalf("a $0 call: status %d, want 201", got)
	}
	rows := usageRows(t, org, who)
	if len(rows) != 1 {
		t.Fatalf("a $0 call wrote %d row(s), want 1", len(rows))
	}
	r := rows[0]
	if r.Amount != 0 || r.Project != "p1" || r.Service != "chat" {
		t.Errorf("row amount %d project %q service %q; want 0, p1, chat", r.Amount, r.Project, r.Service)
	}
	if r.Metadata["model"] != "zen5" || metaInt(r.Metadata["totalTokens"]) != 15 ||
		metaInt(r.Metadata["costMicros"]) != 1500 || r.Metadata["paidBy"] != "hanzo" {
		t.Errorf("row metadata %v; want model zen5, 15 tokens, costMicros 1500, paidBy hanzo", r.Metadata)
	}
}

// Sub-cent calls each write a row and debit, together, exactly the whole cents
// their micros add up to — no call rounded away, no cent charged twice.
func TestSubCentCallsAccumulateExactly(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("usage-subcent")
	const who = "usage-subcent/bob"
	for i := 0; i < 7; i++ { // 7 × 0.3¢ = 2.1¢
		if got := recordUsage(t, org, fmt.Sprintf(`{"user":%q,"amountMicros":3000,"requestId":"sc-%d"}`, who, i)); got != http.StatusCreated {
			t.Fatalf("call %d: status %d, want 201", i, got)
		}
	}
	rows := usageRows(t, org, who)
	if len(rows) != 7 || centsOfRows(rows) != 2 {
		t.Fatalf("7 × 3000µ$: %d row(s) debiting %d¢; want 7 rows debiting 2¢", len(rows), centsOfRows(rows))
	}
	for i := 7; i < 10; i++ { // 10 × 0.3¢ = 3¢
		recordUsage(t, org, fmt.Sprintf(`{"user":%q,"amountMicros":3000,"requestId":"sc-%d"}`, who, i))
	}
	if rows = usageRows(t, org, who); len(rows) != 10 || centsOfRows(rows) != 3 {
		t.Fatalf("10 × 3000µ$: %d row(s) debiting %d¢; want 10 rows debiting 3¢", len(rows), centsOfRows(rows))
	}
	var micros int64
	for _, r := range rows {
		micros += metaInt(r.Metadata["amountMicros"])
	}
	if micros != 30000 {
		t.Fatalf("rows record %dµ$, want 30000", micros)
	}
}

// Concurrent sub-cent calls still debit floor(total) exactly.
func TestConcurrentSubCentCallsAccumulateExactly(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("usage-race")
	const who, n = "usage-race/carol", 40
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			recordUsage(t, org, fmt.Sprintf(`{"user":%q,"amountMicros":2500,"requestId":"race-%d"}`, who, i))
		}(i)
	}
	wg.Wait()
	rows := usageRows(t, org, who)
	if len(rows) != n || centsOfRows(rows) != n*2500/10000 {
		t.Fatalf("%d × 2500µ$: %d row(s) debiting %d¢; want %d rows debiting %d¢", n, len(rows), centsOfRows(rows), n, n*2500/10000)
	}
}

func TestUsageRefusesWhatCannotBeRecorded(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("usage-refuse")
	for _, body := range []string{
		`{"user":"usage-refuse/a","amount":1,"paidBy":"free"}`,
		`{"user":"usage-refuse/a","amount":-1}`,
		`{"user":"usage-refuse/a","amountMicros":-5}`,
		`{"user":"usage-refuse/a","costMicros":-5}`,
		`{"amount":1}`,
	} {
		if got := recordUsage(t, org, body); got != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", body, got)
		}
	}
	if rows := usageRows(t, org, "usage-refuse/a"); len(rows) != 0 {
		t.Fatalf("refused calls wrote %d row(s)", len(rows))
	}
}

// ZAP billing.recordUsage is the same write: it honours amountMicros and records
// a $0 call.
func TestZapRecordUsageIsTheSameWrite(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("usage-zap")
	const who = "usage-zap/dan"
	for _, params := range []string{
		`{"user":"` + who + `","amountMicros":25000,"requestId":"z-1"}`,
		`{"user":"` + who + `","amount":0,"costMicros":900,"paidBy":"plan","requestId":"z-2"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/v1/billing/zap", bytes.NewBufferString(`{"method":"billing.recordUsage","id":"1","params":`+params+`}`))
		req.Header.Set("Content-Type", "application/json")
		w := driveSeeded(capSeed(org), "/v1/billing/zap", req, ZapDispatch)
		out := jsonBody(t, w)
		if out["error"] != nil {
			t.Fatalf("zap %s: %v", params, out["error"])
		}
	}
	rows := usageRows(t, org, who)
	if len(rows) != 2 || centsOfRows(rows) != 2 {
		t.Fatalf("zap wrote %d row(s) debiting %d¢; want 2 rows debiting 2¢ (2.5¢ + $0)", len(rows), centsOfRows(rows))
	}
}

// A DNS usage batch is a row too, an empty one included: it says the zone was
// served and reported, which nothing else records.
func TestAnEmptyDNSBatchWritesAnEvent(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("dns-empty")
	post := func(body string) int {
		req := httptest.NewRequest(http.MethodPost, "/v1/dns/usage", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		return driveSeeded(capSeed(org), "/v1/dns/usage", req, RecordDNSUsage).StatusCode
	}
	if got := post(`{"zone":"example.com","user":"dns-empty/zed","queries":0,"errors":3}`); got != http.StatusCreated {
		t.Fatalf("empty batch: status %d, want 201", got)
	}
	if got := post(`{"zone":"example.com","user":"dns-empty/zed","queries":-1}`); got != http.StatusBadRequest {
		t.Fatalf("negative batch: status %d, want 400", got)
	}
	d := datastore.New(nscontext.WithNamespace(context.Background(), org.Name))
	evts := make([]*meter.MeterEvent, 0)
	if _, err := meter.QueryEvents(d).Ancestor(d.NewKey("synckey", "", 1, nil)).Filter("UserId=", "dns-empty/zed").GetAll(&evts); err != nil {
		t.Fatal(err)
	}
	if len(evts) != 1 || evts[0].Value != 0 || metaInt(evts[0].Dimensions["errors"]) != 3 {
		t.Fatalf("events %d (%+v); want one empty batch carrying its 3 errors", len(evts), evts)
	}
}
