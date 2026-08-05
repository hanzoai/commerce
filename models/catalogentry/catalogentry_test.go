package catalogentry

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/test/ae"
)

func sysDB(c context.Context) *datastore.Datastore {
	return SystemDB(c)
}

func TestSeed_IdempotentAndComplete(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	created, err := Seed(db)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	rows, _ := HanzoSeedRows()
	if len(rows) == 0 {
		t.Fatal("embedded snapshot is empty")
	}
	if created != len(rows) {
		t.Fatalf("first seed created %d, want %d (all snapshot rows)", created, len(rows))
	}

	created2, err := Seed(db)
	if err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	if created2 != 0 {
		t.Fatalf("re-seed created %d, want 0 (idempotent)", created2)
	}
}

// TestProject_ConformsToContract checks the projection matches the
// @hanzo/products CatalogEntry contract for known seeded entries.
func TestProject_ConformsToContract(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)
	if _, err := Seed(db); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cat, err := Project(db, "hanzo")
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if cat.Brand != "hanzo" || len(cat.Categories) != 10 {
		t.Fatalf("hanzo: brand=%s cats=%d, want hanzo/10", cat.Brand, len(cat.Categories))
	}
	if cat.Categories[0].Label != "AI" || cat.Categories[0].ID != "ai" {
		t.Fatalf("first category = %+v, want {ai, AI, 0}", cat.Categories[0])
	}

	byID := map[string]Item{}
	for _, p := range cat.Products {
		byID[p.ID] = p
		if p.IconKey == "" {
			t.Fatalf("%s: empty iconKey", p.ID)
		}
		if p.BrandColor == "" {
			t.Fatalf("%s: empty brandColor (must be a swatch key)", p.ID)
		}
		if p.ID != p.Slug {
			t.Fatalf("%s: id != slug (%s)", p.ID, p.Slug)
		}
		if p.Route == "" {
			t.Fatalf("%s: empty route", p.ID)
		}
		if len(p.ApiPath) < 3 || p.ApiPath[:3] != "/v1" {
			t.Fatalf("%s: apiPath %q not /v1-prefixed", p.ID, p.ApiPath)
		}
	}

	m, ok := byID["models"]
	if !ok {
		t.Fatal("seeded entry 'models' missing from projection")
	}
	if m.Name != "Models" || m.Category != "AI" || m.IconKey != "Brain" {
		t.Fatalf("models = %+v, want Models/AI/Brain", m)
	}
	if m.BrandColor != "violet" { // swatch KEY, not hex
		t.Fatalf("models.brandColor = %q, want swatch key 'violet'", m.BrandColor)
	}
	if m.Route != "/models" || m.DocsUrl != "https://docs.hanzo.ai/docs/services/models" {
		t.Fatalf("models route/docs = %q / %q", m.Route, m.DocsUrl)
	}
	// Every capability carries the enriched taxonomy fields.
	if m.ApiRoute != "api.hanzo.ai/v1/models" {
		t.Fatalf("models.apiRoute = %q, want api.hanzo.ai/v1/models", m.ApiRoute)
	}
	if m.GithubUrl == "" {
		t.Fatalf("models.githubUrl empty")
	}
	// models has a real public price sourced from the pricing table.
	if m.Pricing == nil || m.Pricing.PublicPrice == "" || m.Pricing.PublicPrice == "TODO" {
		t.Fatalf("models.pricing missing a real public price: %+v", m.Pricing)
	}

	// machines carries a fully-real private economics block (cost + margin).
	// The public projection must NEVER expose it: Item has no Private field, so
	// this is a compile-time guarantee — assert the raw entry instead.
	mach := New(db)
	if ok, _ := mach.Query().Filter("Slug=", "machines").Get(); ok {
		if mach.Private == nil || mach.Private.MarginPct == nil {
			t.Fatalf("machines private economics missing: %+v", mach.Private)
		}
	}
}

// TestProject_CategoryScopedByBrand proves scoping is by CATEGORY (matching
// @hanzo/products catalogForBrand): a sovereign brand (lux) sees only entries in
// its 4 categories, never an AI/Compute/etc. entry.
func TestProject_CategoryScopedByBrand(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)
	if _, err := Seed(db); err != nil {
		t.Fatalf("seed: %v", err)
	}

	lux, err := Project(db, "lux")
	if err != nil {
		t.Fatalf("project lux: %v", err)
	}
	if len(lux.Categories) != 4 {
		t.Fatalf("lux categories = %d, want 4", len(lux.Categories))
	}
	allowed := map[string]bool{"Web3": true, "Network": true, "Security": true, "Dev": true}
	for _, p := range lux.Products {
		if !allowed[p.Category] {
			t.Fatalf("lux projection leaked %q in non-lux category %q", p.ID, p.Category)
		}
		if p.ID == "models" {
			t.Fatal("lux projection contains an AI-category entry (models)")
		}
	}
	hanzo, _ := Project(db, "hanzo")
	if len(hanzo.Products) <= len(lux.Products) {
		t.Fatalf("hanzo products (%d) should exceed lux (%d)", len(hanzo.Products), len(lux.Products))
	}
}

// asFloat coerces a JSON-decoded Metadata value (numbers round-trip as float64
// through the SQLite Metadata_ blob) to float64 for comparison.
func asFloat(t *testing.T, v interface{}) float64 {
	t.Helper()
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("metadata value %#v is not a number", v)
	}
	return f
}

// TestSeedInfra_IdempotentAndComplete proves the infra-tier seed creates exactly
// the embedded rows (10 cloud + 3 gpu + 3 datastore), is idempotent, and that the
// count-gated SeedInfraIfEmpty seeds once then no-ops.
func TestSeedInfra_IdempotentAndComplete(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	rows, err := InfraSeedRows()
	if err != nil {
		t.Fatalf("infra seed rows: %v", err)
	}
	if len(rows) != 16 {
		t.Fatalf("embedded infra snapshot has %d rows, want 16 (10 cloud + 3 gpu + 3 datastore)", len(rows))
	}

	// The catalog sells no free tier. "starter" was a phantom rung — advertised
	// on the public price list, freeTier:true, and never sellable (no Stripe
	// price ref anywhere in the ladder). It is gone; this keeps it gone.
	for _, r := range rows {
		if r.Slug == "cloud-starter" {
			t.Fatalf("cloud-starter is a phantom plan and must not be seeded")
		}
		if r.Metadata["freeTier"] == true {
			t.Fatalf("%s: no infra tier is a free tier", r.Slug)
		}
	}

	// The gate seeds every row when the infra scope is empty (this test's db
	// starts empty), independently of the hanzo product snapshot.
	created, err := SeedInfraIfEmpty(db)
	if err != nil {
		t.Fatalf("seed infra: %v", err)
	}
	if created != len(rows) {
		t.Fatalf("first infra seed created %d, want %d", created, len(rows))
	}

	// Idempotent: the gate no-ops once the tiers exist, and so does the
	// per-row seed (never clobbers, never duplicates).
	if n, err := SeedInfraIfEmpty(db); err != nil || n != 0 {
		t.Fatalf("SeedInfraIfEmpty on populated db = (%d, %v), want (0, nil)", n, err)
	}
	if n, err := SeedInfra(db); err != nil || n != 0 {
		t.Fatalf("SeedInfra re-run = (%d, %v), want (0, nil) (idempotent)", n, err)
	}
}

// TestSeedInfra_ProjectionAndValues proves the infra tiers project under
// ?brand=infra with their category, PriceCents, and structured Metadata intact —
// and that they never leak into a per-brand (hanzo) catalog.
func TestSeedInfra_ProjectionAndValues(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)
	if _, err := SeedInfra(db); err != nil {
		t.Fatalf("seed infra: %v", err)
	}

	cat, err := Project(db, "infra")
	if err != nil {
		t.Fatalf("project infra: %v", err)
	}
	if len(cat.Categories) != 3 {
		t.Fatalf("infra categories = %d, want 3", len(cat.Categories))
	}
	if len(cat.Products) != 16 {
		t.Fatalf("infra products = %d, want 16", len(cat.Products))
	}

	bySlug := map[string]Item{}
	counts := map[string]int{}
	for _, p := range cat.Products {
		bySlug[p.Slug] = p
		counts[p.Category]++
		if p.PriceCents <= 0 {
			t.Fatalf("%s: PriceCents must be a positive display price, got %d", p.Slug, p.PriceCents)
		}
		if p.Metadata == nil {
			t.Fatalf("%s: Metadata must project the structured spec, got nil", p.Slug)
		}
	}
	if counts["cloud"] != 10 || counts["gpu"] != 3 || counts["datastore"] != 3 {
		t.Fatalf("category counts = %v, want cloud:10 gpu:3 datastore:3", counts)
	}

	// Cloud: display price = monthly * 100; Metadata carries the VM spec verbatim.
	// builder is the entry rung — the catalog starts at a plan we actually sell.
	if _, ok := bySlug["cloud-starter"]; ok {
		t.Fatalf("cloud-starter is a phantom plan and must not project")
	}
	builder := bySlug["cloud-builder"]
	if builder.Category != "cloud" || builder.PriceCents != 1000 {
		t.Fatalf("cloud-builder = {cat:%s cents:%d}, want {cloud 1000}", builder.Category, builder.PriceCents)
	}
	if asFloat(t, builder.Metadata["priceMonthly"]) != 10 || asFloat(t, builder.Metadata["vcpus"]) != 2 ||
		asFloat(t, builder.Metadata["maxVMs"]) != 5 || builder.Metadata["cpuType"] != "shared" {
		t.Fatalf("cloud-builder metadata spec wrong: %#v", builder.Metadata)
	}

	// GPU: display price = hourly * 100; Metadata carries gpu/vram/price.
	gpu := bySlug["gpu-standard"]
	if gpu.Category != "gpu" || gpu.PriceCents != 348 {
		t.Fatalf("gpu-standard = {cat:%s cents:%d}, want {gpu 348}", gpu.Category, gpu.PriceCents)
	}
	if gpu.Metadata["gpu"] != "1x H100" || gpu.Metadata["vram"] != "80 GB" || asFloat(t, gpu.Metadata["price"]) != 3.48 {
		t.Fatalf("gpu-standard metadata wrong: %#v", gpu.Metadata)
	}

	// Datastore: display price = monthly * 100; Metadata carries the tier spec
	// AND the shared usage rates.
	ds := bySlug["datastore-basic"]
	if ds.Category != "datastore" || ds.PriceCents != 6652 {
		t.Fatalf("datastore-basic = {cat:%s cents:%d}, want {datastore 6652}", ds.Category, ds.PriceCents)
	}
	if asFloat(t, ds.Metadata["replicas"]) != 1 || asFloat(t, ds.Metadata["priceHourly"]) != 0.0922 {
		t.Fatalf("datastore-basic spec wrong: %#v", ds.Metadata)
	}
	usage, ok := ds.Metadata["usage"].(map[string]interface{})
	if !ok {
		t.Fatalf("datastore-basic must carry the usage block, got %#v", ds.Metadata["usage"])
	}
	storage, ok := usage["storage"].(map[string]interface{})
	if !ok || asFloat(t, storage["pricePerGBMonth"]) != 0.0247 {
		t.Fatalf("datastore usage.storage rate wrong: %#v", usage["storage"])
	}

	// Isolation: the infra tiers must NOT leak into the per-brand hanzo catalog.
	hanzo, err := Project(db, "hanzo")
	if err != nil {
		t.Fatalf("project hanzo: %v", err)
	}
	if len(hanzo.Categories) != 10 {
		t.Fatalf("hanzo categories = %d, want 10 (infra scope must not widen it)", len(hanzo.Categories))
	}
	for _, p := range hanzo.Products {
		switch p.Category {
		case "cloud", "gpu", "datastore":
			t.Fatalf("infra tier %q leaked into the hanzo catalog under %q", p.Slug, p.Category)
		}
	}
}

func TestProject_UnpublishedExcluded(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	e := New(db)
	e.Slug = "hidden"
	e.Name = "Hidden"
	e.Category = "AI"
	e.IconKey = "Lock"
	e.BrandColor = "slate"
	e.Published = false
	if err := e.Create(); err != nil {
		t.Fatalf("create: %v", err)
	}

	cat, err := Project(db, "hanzo")
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	for _, p := range cat.Products {
		if p.Slug == "hidden" {
			t.Fatal("unpublished entry appeared in the public projection")
		}
	}
}
