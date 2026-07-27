package catalogentry

import (
	"os"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/test/ae"
)

func orRow(slug, inCost string) ModelRow {
	return ModelRow{
		Slug:  slug,
		Name:  slug,
		Costs: []Rate{{Key: RateIn, Unit: UnitMTok, Cost: inCost}},
		Spec: ModelSpec{
			Vendor: "Test", Family: FamilyThirdParty,
			Serves: ServesOpenRouter, Upstream: slug, Enabled: true,
		},
	}
}

func bySlug(t *testing.T, db *datastore.Datastore, slug string) *CatalogEntry {
	t.Helper()
	e := New(db)
	ok, err := e.Query().Filter("Slug=", slug).Get()
	if err != nil || !ok {
		t.Fatalf("slug %q not found (err=%v)", slug, err)
	}
	return e
}

func inRate(t *testing.T, e *CatalogEntry) Rate {
	t.Helper()
	for _, r := range RatesOf(e) {
		if r.Key == RateIn {
			return r
		}
	}
	t.Fatalf("%s has no input rate", e.Slug)
	return Rate{}
}

// ─── the non-negotiable ──────────────────────────────────────────────────────

// TestRefresh_UpstreamCostMoveNeverRepricesTheCustomer is the whole point. An
// upstream doubles its cost; the customer's price must not move by a digit, and
// the change must surface as a margin that collapsed.
func TestRefresh_UpstreamCostMoveNeverRepricesTheCustomer(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	if _, err := Refresh(db, ServesOpenRouter, []ModelRow{orRow("openai/gpt-5", "1.25")}); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	born := inRate(t, bySlug(t, db, "openai/gpt-5"))
	if born.Price != "1.5" { // 1.25 × the 1.20 default markup, exact
		t.Fatalf("price at birth = %q, want 1.5 — a new row must arrive priced", born.Price)
	}

	// The upstream doubles.
	if _, err := Refresh(db, ServesOpenRouter, []ModelRow{orRow("openai/gpt-5", "2.50")}); err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	e := bySlug(t, db, "openai/gpt-5")
	after := inRate(t, e)

	if after.Cost != "2.50" {
		t.Fatalf("cost = %q, want 2.50 — a sync owns cost", after.Cost)
	}
	if after.Price != born.Price {
		t.Fatalf("PRICE MOVED %q → %q. An upstream cost change must never re-price a customer-facing SKU.",
			born.Price, after.Price)
	}
	margin := RateMarginPct(after, e.Markup)
	if margin == nil || *margin >= 0 {
		t.Fatalf("margin = %v, want a visible negative (price 1.5 now under cost 2.50)", margin)
	}
}

// TestRefresh_AdminPriceSurvives proves the other direction: a price a human set
// in admin.hanzo.ai is not reverted by the next scheduled run.
func TestRefresh_AdminPriceSurvives(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	if _, err := Refresh(db, ServesOpenRouter, []ModelRow{orRow("openai/gpt-5", "1.25")}); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	e := bySlug(t, db, "openai/gpt-5")
	e.Rates[0].Price = "9.99" // an admin decides
	if err := e.Update(); err != nil {
		t.Fatalf("admin edit: %v", err)
	}

	if _, err := Refresh(db, ServesOpenRouter, []ModelRow{orRow("openai/gpt-5", "1.30")}); err != nil {
		t.Fatalf("re-refresh: %v", err)
	}
	after := inRate(t, bySlug(t, db, "openai/gpt-5"))
	if after.Price != "9.99" {
		t.Fatalf("admin price = %q, want 9.99 — a sync must not clobber a human's edit", after.Price)
	}
	if after.Cost != "1.30" {
		t.Fatalf("cost = %q, want 1.30", after.Cost)
	}
}

// ─── withdrawal: marked, never deleted ───────────────────────────────────────

func TestRefresh_WithdrawnModelIsMarkedNotDeleted(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	both := []ModelRow{orRow("openai/gpt-5", "1.25"), orRow("openai/gpt-4", "10")}
	if _, err := Refresh(db, ServesOpenRouter, both); err != nil {
		t.Fatalf("first: %v", err)
	}

	res, err := Refresh(db, ServesOpenRouter, []ModelRow{orRow("openai/gpt-5", "1.25")})
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if len(res.Withdrawn) != 1 || res.Withdrawn[0] != "openai/gpt-4" {
		t.Fatalf("withdrawn = %v, want exactly [openai/gpt-4]", res.Withdrawn)
	}

	gone := bySlug(t, db, "openai/gpt-4") // still THERE — billing history references it
	if gone.Spec.Enabled {
		t.Fatal("a withdrawn model must stop being routable")
	}
	if gone.Spec.Unavailable != WithdrawnUpstream {
		t.Fatalf("reason = %q, want an honest stated reason", gone.Spec.Unavailable)
	}
	if !gone.Published || inRate(t, gone).Price == "" {
		t.Fatal("a withdrawn model stays published and priced — visible with a reason, never hidden")
	}

	// A second run withdraws nothing new: withdrawal is idempotent.
	again, err := Refresh(db, ServesOpenRouter, []ModelRow{orRow("openai/gpt-5", "1.25")})
	if err != nil {
		t.Fatalf("re-withdraw: %v", err)
	}
	if len(again.Withdrawn) != 0 {
		t.Fatalf("withdrawn again = %v, want none", again.Withdrawn)
	}

	// And it comes back when the upstream publishes it again.
	back, err := Refresh(db, ServesOpenRouter, both)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if len(back.Restored) != 1 || back.Restored[0] != "openai/gpt-4" {
		t.Fatalf("restored = %v, want [openai/gpt-4]", back.Restored)
	}
	if e := bySlug(t, db, "openai/gpt-4"); !e.Spec.Enabled || e.Spec.Unavailable != "" {
		t.Fatalf("restored entry = enabled %v / reason %q", e.Spec.Enabled, e.Spec.Unavailable)
	}
}

// TestRefresh_DoesNotReverseAHumanDisable: an operator who turned a model off by
// hand keeps that decision. Only a model the sync itself withdrew is restored —
// the upstream never had the authority to reverse an operator.
func TestRefresh_DoesNotReverseAHumanDisable(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	rows := []ModelRow{orRow("openai/gpt-5", "1.25")}
	if _, err := Refresh(db, ServesOpenRouter, rows); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	e := bySlug(t, db, "openai/gpt-5")
	e.Spec.Enabled, e.Spec.Unavailable = false, "disabled by an operator"
	if err := e.Update(); err != nil {
		t.Fatalf("operator disable: %v", err)
	}

	res, err := Refresh(db, ServesOpenRouter, rows)
	if err != nil {
		t.Fatalf("re-refresh: %v", err)
	}
	if len(res.Restored) != 0 {
		t.Fatalf("restored = %v, want none", res.Restored)
	}
	after := bySlug(t, db, "openai/gpt-5")
	if after.Spec.Enabled || after.Spec.Unavailable != "disabled by an operator" {
		t.Fatalf("operator decision was reversed: enabled=%v reason=%q", after.Spec.Enabled, after.Spec.Unavailable)
	}
}

// ─── the fail-safe ───────────────────────────────────────────────────────────

// TestRefresh_EmptyUpstreamLeavesTheCatalogIntact: zero models is
// indistinguishable from an upstream we failed to reach. Believing it would
// withdraw the entire catalog in one run.
func TestRefresh_EmptyUpstreamLeavesTheCatalogIntact(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	if _, err := Refresh(db, ServesOpenRouter, []ModelRow{orRow("openai/gpt-5", "1.25")}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := Refresh(db, ServesOpenRouter, nil); err != ErrEmptyUpstream {
		t.Fatalf("empty upstream err = %v, want ErrEmptyUpstream", err)
	}
	if e := bySlug(t, db, "openai/gpt-5"); !e.Spec.Enabled {
		t.Fatal("a refused sync must leave the last good catalog exactly as it was")
	}
}

// TestRefresh_ScopedByServes: one upstream's outage must never withdraw another
// family's models. Enso and Zen are ours; OpenRouter going dark cannot touch them.
func TestRefresh_ScopedByServes(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	if _, err := UpsertModels(db, []ModelRow{{
		Slug: "enso", Name: "Enso",
		Spec: ModelSpec{Family: FamilyEnso, Serves: "enso", Enabled: true},
	}}); err != nil {
		t.Fatalf("seed enso: %v", err)
	}
	if _, err := Refresh(db, ServesOpenRouter, []ModelRow{orRow("openai/gpt-5", "1.25")}); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if e := bySlug(t, db, "enso"); !e.Spec.Enabled {
		t.Fatal("an OpenRouter refresh withdrew an Enso model — the sweep must be scoped by serves")
	}
}

// TestRefresh_IdempotentOnState: the same upstream payload yields the same
// catalog, and a repeat run withdraws and restores nothing.
func TestRefresh_IdempotentOnState(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	rows := []ModelRow{orRow("openai/gpt-5", "1.25"), orRow("anthropic/claude-opus-5", "5")}
	first, err := Refresh(db, ServesOpenRouter, rows)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if first.Created != 2 {
		t.Fatalf("first = %+v, want created 2", first)
	}
	second, err := Refresh(db, ServesOpenRouter, rows)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.Created != 0 || len(second.Withdrawn) != 0 || len(second.Restored) != 0 {
		t.Fatalf("second = %+v, want no creates, no withdrawals, no restores", second)
	}
	if n, _ := Query(db).Count(); n != 2 {
		t.Fatalf("catalog holds %d rows, want 2 — no duplicates", n)
	}
	// Prices are untouched by a repeat run.
	if got := inRate(t, bySlug(t, db, "anthropic/claude-opus-5")).Price; got != "6" {
		t.Fatalf("price after a repeat run = %q, want 6", got)
	}
}

// ─── the upstream decoder, against a payload captured from live OpenRouter ───

func TestDecodeOpenRouter_LiveFixture(t *testing.T) {
	body, err := os.ReadFile("testdata/openrouter-models.json")
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	rows, err := DecodeOpenRouter(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	byID := map[string]ModelRow{}
	for _, r := range rows {
		byID[r.Slug] = r
	}

	opus, ok := byID["anthropic/claude-opus-5"]
	if !ok {
		t.Fatal("claude-opus-5 missing from the decode")
	}
	if opus.Spec.Vendor != "Anthropic" || opus.Name != "Claude Opus 5" {
		t.Fatalf("opus = vendor %q / name %q, want Anthropic / Claude Opus 5", opus.Spec.Vendor, opus.Name)
	}
	if opus.Spec.ContextWindow != 1000000 || opus.Spec.Family != FamilyThirdParty {
		t.Fatalf("opus spec = %+v", opus.Spec)
	}
	costs := map[string]string{}
	for _, r := range opus.Costs {
		costs[r.Key] = r.Cost
	}
	// Per-token upstream figures, shifted six places exactly.
	if costs[RateIn] != "5" || costs[RateOut] != "25" || costs[RateCacheRead] != "0.5" {
		t.Fatalf("opus costs = %v, want in 5 / out 25 / cacheRead 0.5 per MTok", costs)
	}

	// The naming rule: the PARTY, never party+product.
	llama, ok := byID["meta-llama/llama-4-maverick"]
	if !ok {
		t.Fatal("llama-4-maverick missing")
	}
	if llama.Spec.Vendor != "Meta" {
		t.Fatalf("vendor = %q, want Meta (never \"Meta Llama\")", llama.Spec.Vendor)
	}
	if llama.Name != "Llama 4 Maverick" {
		t.Fatalf("name = %q, want the product alone", llama.Name)
	}

	// Sub-cent precision survives exactly — no float anywhere in the path.
	for _, r := range byID["deepseek/deepseek-v4-pro"].Costs {
		if r.Key == RateIn && r.Cost != "0.435" {
			t.Fatalf("deepseek input = %q, want 0.435", r.Cost)
		}
	}

	// A decoder states COST and never price. UpsertModels enforces it too; this
	// proves the decoder does not even try.
	for _, r := range rows {
		for _, cost := range r.Costs {
			if cost.Price != "" {
				t.Fatalf("%s: the decoder set a PRICE (%q). An upstream supplies cost only.", r.Slug, cost.Price)
			}
		}
	}
}

// TestDecodeOpenRouter_OneVendorNamePerNamespace: a vendor is a PARTY, so every
// model in a namespace must carry the identical vendor name. Deriving it from
// each model's own display string yields "OpenAI" and "Openai" for one vendor,
// which makes grouping by vendor quietly wrong.
func TestDecodeOpenRouter_OneVendorNamePerNamespace(t *testing.T) {
	body, err := os.ReadFile("testdata/openrouter-models.json")
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	rows, err := DecodeOpenRouter(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	seen := map[string]string{}
	for _, r := range rows {
		ns := Namespace(r.Slug)
		if prev, ok := seen[ns]; ok && prev != r.Spec.Vendor {
			t.Fatalf("namespace %q yields two vendor names: %q and %q", ns, prev, r.Spec.Vendor)
		}
		seen[ns] = r.Spec.Vendor
		if r.Spec.Vendor == "" {
			t.Fatalf("%s has no vendor", r.Slug)
		}
	}
	// A floating alias is the same party as its pinned namespace.
	if got := Namespace("~openai/gpt-latest"); got != "openai" {
		t.Fatalf("Namespace(~openai/…) = %q, want openai", got)
	}
}

// TestDecodeOpenRouter_RefusesGarbage: a half-read catalog looks exactly like an
// upstream that shrank, and the sync's answer to a shrinking upstream is to
// withdraw models. The two must be told apart before that decision is reached.
func TestDecodeOpenRouter_RefusesGarbage(t *testing.T) {
	for _, bad := range []string{`{"data":[]}`, `not json`, `{"data":[{"id":""}]}`} {
		if _, err := DecodeOpenRouter([]byte(bad)); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
