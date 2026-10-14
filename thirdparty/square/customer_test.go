package square

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/square/square-go-sdk/v3/core"
	"github.com/square/square-go-sdk/v3/customers/client"
	"github.com/square/square-go-sdk/v3/option"
)

// fakeSquare answers Square's customer endpoints over HTTP: each path replies
// with reply[path], and every request body is kept under its path. Both are
// read and changed under mu, the handler's and the test's goroutines alike.
type fakeSquare struct {
	mu    sync.Mutex
	reply map[string]string
	seen  map[string][]map[string]any
}

func (f *fakeSquare) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	err := json.NewDecoder(r.Body).Decode(&body)
	f.mu.Lock()
	defer f.mu.Unlock()
	if err != nil {
		body = map[string]any{"undecodable": err.Error()}
	}
	if f.seen == nil {
		f.seen = map[string][]map[string]any{}
	}
	f.seen[r.URL.Path] = append(f.seen[r.URL.Path], body)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(f.reply[r.URL.Path]))
}

func (f *fakeSquare) answer(path, reply string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.reply == nil {
		f.reply = map[string]string{}
	}
	f.reply[path] = reply
}

func (f *fakeSquare) sent(path string) []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.seen[path]
}

// squareAt is a processor whose customer calls go to f.
func squareAt(t *testing.T, f *fakeSquare) *SquareProcessor {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	sp := NewProcessor(Config{AccessToken: "tok", LocationID: "loc", Environment: "sandbox"})
	sp.customersClient = client.NewClient(core.NewRequestOptions(option.WithToken("tok"), option.WithBaseURL(srv.URL)))
	return sp
}

// TestCreateCustomer_CarriesReferenceAndKey: a create names its reference and
// carries an idempotency key within Square's limit, the same key for the same
// (reference, email) and another key for another email.
func TestCreateCustomer_CarriesReferenceAndKey(t *testing.T) {
	f := &fakeSquare{}
	f.answer("/v2/customers", `{"customer":{"id":"cust_1"}}`)
	sp := squareAt(t, f)
	ctx := context.Background()
	meta := func() map[string]interface{} {
		return map[string]interface{}{"reference": "acme/acme/alice", "note": "Hanzo billing customer acme/alice"}
	}

	for _, email := range []string{"alice@acme.test", "alice@acme.test", "other@acme.test"} {
		id, err := sp.CreateCustomer(ctx, email, "acme/alice", meta())
		if err != nil || id != "cust_1" {
			t.Fatalf("CreateCustomer(%s) = %q, %v", email, id, err)
		}
	}

	sent := f.sent("/v2/customers")
	if len(sent) != 3 {
		t.Fatalf("creates sent=%d, want 3", len(sent))
	}
	for i, body := range sent {
		if body["reference_id"] != "acme/acme/alice" {
			t.Fatalf("create %d reference_id=%v, want acme/acme/alice", i, body["reference_id"])
		}
		key, _ := body["idempotency_key"].(string)
		if key == "" || len(key) > keyMax {
			t.Fatalf("create %d idempotency_key=%q, want 1..%d characters", i, key, keyMax)
		}
	}
	if sent[0]["idempotency_key"] != sent[1]["idempotency_key"] {
		t.Fatalf("same reference and email sent keys %v and %v, want one", sent[0]["idempotency_key"], sent[1]["idempotency_key"])
	}
	if sent[0]["idempotency_key"] == sent[2]["idempotency_key"] {
		t.Fatal("another email sent the same key, which Square would refuse with a different body")
	}
}

// TestFindCustomer_SearchesTheReferenceExactly: the search asks for one
// customer whose reference_id is exactly the reference, oldest first, and
// answers "" when Square holds none.
func TestFindCustomer_SearchesTheReferenceExactly(t *testing.T) {
	f := &fakeSquare{}
	f.answer("/v2/customers/search", `{"customers":[{"id":"cust_old"}]}`)
	sp := squareAt(t, f)
	ctx := context.Background()

	id, err := sp.FindCustomer(ctx, "acme/acme")
	if err != nil || id != "cust_old" {
		t.Fatalf("FindCustomer = %q, %v, want cust_old", id, err)
	}
	body := f.sent("/v2/customers/search")[0]
	if body["limit"] != float64(1) {
		t.Fatalf("limit=%v, want 1", body["limit"])
	}
	query, _ := body["query"].(map[string]any)
	filter, _ := query["filter"].(map[string]any)
	ref, _ := filter["reference_id"].(map[string]any)
	if ref["exact"] != "acme/acme" || ref["fuzzy"] != nil {
		t.Fatalf("reference filter=%v, want exact acme/acme", ref)
	}
	sort, _ := query["sort"].(map[string]any)
	if sort["field"] != "CREATED_AT" {
		t.Fatalf("sort=%v, want CREATED_AT", sort)
	}

	f.answer("/v2/customers/search", `{}`)
	if id, err := sp.FindCustomer(ctx, "acme/acme"); err != nil || id != "" {
		t.Fatalf("FindCustomer with no match = %q, %v, want \"\"", id, err)
	}
}

// TestCustomerReference_FitsTheLimit: a reference over Square's limit is sent
// as the same fitted value by the create and the search, so they still meet.
func TestCustomerReference_FitsTheLimit(t *testing.T) {
	f := &fakeSquare{}
	f.answer("/v2/customers", `{"customer":{"id":"cust_long"}}`)
	f.answer("/v2/customers/search", `{}`)
	sp := squareAt(t, f)
	ctx := context.Background()
	long := "acme/acme/" + strings.Repeat("u", 120)

	if _, err := sp.CreateCustomer(ctx, "u@acme.test", "acme/u", map[string]interface{}{"reference": long}); err != nil {
		t.Fatalf("CreateCustomer: %v", err)
	}
	if _, err := sp.FindCustomer(ctx, long); err != nil {
		t.Fatalf("FindCustomer: %v", err)
	}
	made, _ := f.sent("/v2/customers")[0]["reference_id"].(string)
	query, _ := f.sent("/v2/customers/search")[0]["query"].(map[string]any)
	filter, _ := query["filter"].(map[string]any)
	ref, _ := filter["reference_id"].(map[string]any)
	if made == "" || len(made) > refMax || made != ref["exact"] {
		t.Fatalf("created under %q, searched %v, want one value within %d characters", made, ref["exact"], refMax)
	}
}
