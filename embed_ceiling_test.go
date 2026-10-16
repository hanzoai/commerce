package commerce

import (
	"testing"

	"github.com/hanzoai/commerce/models/organization"
)

// A host's declaration maps to a cap: none for "" and Live, Test for Test and
// for anything unrecognised.
func TestCeilingOf(t *testing.T) {
	for in, want := range map[organization.Kind]organization.Kind{
		"":                organization.Live,
		organization.Live: organization.Live,
		organization.Test: organization.Test,
		"production":      organization.Test,
		"sandbox":         organization.Test,
	} {
		if got := ceilingOf(in); got != want {
			t.Errorf("ceilingOf(%q) = %q, want %q", in, got, want)
		}
	}
}
