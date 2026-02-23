package stripe

import (
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
)

func TestStripeSupportedCurrencies(t *testing.T) {
	currencies := StripeSupportedCurrencies()

	if len(currencies) == 0 {
		t.Fatal("StripeSupportedCurrencies returned empty slice")
	}

	// Must include the major currencies.
	required := []currency.Type{
		currency.USD, currency.EUR, currency.GBP, currency.JPY,
	}

	have := make(map[currency.Type]bool)
	for _, c := range currencies {
		have[c] = true
	}

	for _, r := range required {
		if !have[r] {
			t.Errorf("missing required currency %q", r)
		}
	}

	// No duplicates.
	seen := make(map[currency.Type]bool)
	for _, c := range currencies {
		if seen[c] {
			t.Errorf("duplicate currency %q", c)
		}
		seen[c] = true
	}
}
