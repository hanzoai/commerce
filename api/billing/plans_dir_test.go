package billing

import (
	"os"
	"path/filepath"
	"testing"
)

// The catalog is DATA the operator states, and the embed is only what ships
// when they state nothing. Three properties make that safe to believe with
// money on the line, and each is one of the ways this could go quietly wrong.

func TestPlansDirIsRead(t *testing.T) {
	// A supplied catalog WINS. Without this the flag is decorative: the service
	// reads the embed, the operator reads their own file, and both are certain
	// they know the price.
	dir := t.TempDir()
	write(t, dir, "subscription.json", `[{"id":"go","name":"Go","category":"personal","priceMonthly":9,"priceAnnual":8.25}]`)

	got := loadPlans(dir, subscriptionJSON, "plans/subscription.json")
	if len(got) != 1 || got[0].Slug != "go" {
		t.Fatalf("supplied catalog not read: %+v", got)
	}
	if got[0].Price != 900 || got[0].PriceAnnual != 825 {
		t.Fatalf("price not taken from the supplied catalog: %d/%d", got[0].Price, got[0].PriceAnnual)
	}
}

func TestPlansDirUnsetUsesEmbed(t *testing.T) {
	// Unset is the ONLY condition that means "use the default", and it must be
	// byte-identical to what shipped before this existed.
	embedded := loadPlansFromEmbed(subscriptionJSON, "plans/subscription.json")
	got := loadPlans("", subscriptionJSON, "plans/subscription.json")
	if len(got) != len(embedded) {
		t.Fatalf("unset changed the catalog: %d vs %d", len(got), len(embedded))
	}
	for i := range got {
		if got[i].Slug != embedded[i].Slug || got[i].Price != embedded[i].Price {
			t.Fatalf("unset changed %s: %d vs %d", got[i].Slug, got[i].Price, embedded[i].Price)
		}
	}
}

func TestPlansDirRefusesABadCatalog(t *testing.T) {
	// It must NOT fall back. Falling back on a malformed file is the worst
	// outcome available: the operator states one catalog, the service sells
	// another, and nothing says so. For a file that decides what customers are
	// charged, that is a silent wrong answer rather than a degradation.
	for name, body := range map[string]string{
		"malformed":  `{ not json`,
		"wrongShape": `{"plans":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "subscription.json", body)
			defer func() {
				if recover() == nil {
					t.Fatal("a bad supplied catalog was accepted")
				}
			}()
			loadPlans(dir, subscriptionJSON, "plans/subscription.json")
		})
	}

	// A named directory with no catalog in it is equally fatal — it means the
	// mount did not land, and serving the embed there would hide that.
	defer func() {
		if recover() == nil {
			t.Fatal("a missing supplied catalog was accepted")
		}
	}()
	loadPlans(t.TempDir(), subscriptionJSON, "plans/subscription.json")
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
