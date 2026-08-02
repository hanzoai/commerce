package plan

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A plan can be RE-DESCRIBED through the admin CRUD, not just repriced.
//
// The display envelope (features, limits, bundles, includedIn) used to live only
// as a packed JSON string inside Metadata, with the packer private to api/billing
// — somewhere this handler could not reach. So a PUT carrying `features` set the
// price and silently discarded the copy. A tier could be moved to $49 while its
// own feature list went on quoting the $20 allowances, and nothing anywhere
// reported a failure. These pin that the wire shape the public read EMITS is the
// wire shape this endpoint ACCEPTS.
func TestUpdateEntry_PersistsFeaturesAndLimits(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	create := []byte(`{"slug":"pro","name":"Pro","price":2000,
	  "features":["old copy"],"limits":{"minSeats":1}}`)
	if code, body := callPlan(t, admin, http.MethodPost, "/v1/plans/entries", create); code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}

	// Reprice AND re-describe in one edit, exactly as the ladder seed does.
	update := []byte(`{"price":4900,
	  "features":["Auto Drive orchestration","Auto Review","1,000 credits/month"],
	  "limits":{"minSeats":2,"includedCreditUsd":49}}`)
	code, body := callPlan(t, admin, http.MethodPut, "/v1/plans/entries/pro", update)
	if code != http.StatusOK {
		t.Fatalf("update: %d %s", code, body)
	}

	// The RESPONSE must carry them — a caller has to be able to see what it wrote.
	var got plan.Plan
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode response: %v (%s)", err, body)
	}
	if len(got.Features) != 3 {
		t.Fatalf("response features = %v, want the 3 sent (they were dropped)", got.Features)
	}
	if got.Limits == nil || got.Limits.MinSeats == nil || *got.Limits.MinSeats != 2 {
		t.Fatalf("response limits = %+v, want minSeats 2", got.Limits)
	}

	// And they must PERSIST — a response echoing the request while the store keeps
	// the old copy is the same bug wearing a disguise.
	code, body = callPlan(t, admin, http.MethodGet, "/v1/plans/entries", nil)
	if code != http.StatusOK {
		t.Fatalf("list: %d %s", code, body)
	}
	var rows []plan.Plan
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatalf("decode list: %v (%s)", err, body)
	}
	var stored *plan.Plan
	for i := range rows {
		if rows[i].Slug == "pro" {
			stored = &rows[i]
		}
	}
	if stored == nil {
		t.Fatal("pro missing from the authority after update")
	}
	if len(stored.Features) != 3 || stored.Features[0] != "Auto Drive orchestration" {
		t.Fatalf("stored features = %v, want the 3 sent", stored.Features)
	}
	if stored.Limits == nil || stored.Limits.MinSeats == nil || *stored.Limits.MinSeats != 2 {
		t.Fatalf("stored limits = %+v, want minSeats 2", stored.Limits)
	}
	if int64(stored.Price) != 4900 {
		t.Fatalf("stored price = %d, want 4900", stored.Price)
	}
}

// A partial edit must not wipe the copy. UpdateEntry loads the row before
// decoding, so an absent field keeps its stored value — the same guarantee that
// already protects Price, extended to the fields beside it.
func TestUpdateEntry_AbsentEnvelopeKeepsStoredCopy(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	create := []byte(`{"slug":"dev","name":"Dev","price":1900,"features":["Terminal-native pair programming"]}`)
	if code, body := callPlan(t, admin, http.MethodPost, "/v1/plans/entries", create); code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}

	// Price-only edit — no `features` key at all.
	code, body := callPlan(t, admin, http.MethodPut, "/v1/plans/entries/dev", []byte(`{"price":2900}`))
	if code != http.StatusOK {
		t.Fatalf("update: %d %s", code, body)
	}
	var got plan.Plan
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v (%s)", err, body)
	}
	if int64(got.Price) != 2900 {
		t.Fatalf("price = %d, want 2900", got.Price)
	}
	if len(got.Features) != 1 || got.Features[0] != "Terminal-native pair programming" {
		t.Fatalf("features = %v, want the stored copy preserved", got.Features)
	}
}
