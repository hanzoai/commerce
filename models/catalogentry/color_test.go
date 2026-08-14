package catalogentry

import "testing"

// A category's accent is part of the category, so the two have to be defined
// over the same set. The failure this prevents is quiet in both directions: a
// category added without an accent is served Color:"" and every surface falls
// back to its own guess (which is how the same group ended up teal in one
// console and indigo in another), and an accent left behind after a category is
// retired is a value nothing reads and nobody notices is stale.
func TestCategoryColors_CoverTheTaxonomy(t *testing.T) {
	for _, label := range canonicalCategories {
		if categoryColors[label] == "" {
			t.Errorf("%q has no accent — it will be served Color:\"\" and each surface will pick its own", label)
		}
	}
	canonical := make(map[string]bool, len(canonicalCategories))
	for _, label := range canonicalCategories {
		canonical[label] = true
	}
	for label := range categoryColors {
		if !canonical[label] {
			t.Errorf("%q has an accent but is not in the taxonomy — a colour for a category nothing serves", label)
		}
	}
}

// Whatever the projection serves is what every consumer copies, so the accent
// has to survive the trip rather than only existing in the map.
func TestProjection_ServesTheAccent(t *testing.T) {
	for _, brand := range []string{"hanzo", "lux"} {
		for _, cat := range categoriesForBrand(brand) {
			if cat.Color == "" {
				t.Errorf("brand %s: category %q projected with no color", brand, cat.Label)
			}
			if cat.Color != categoryColors[cat.Label] {
				t.Errorf("brand %s: category %q color = %q, want %q", brand, cat.Label, cat.Color, categoryColors[cat.Label])
			}
		}
	}
}
