package plan

import (
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A row's reload figures persist with its limits: a present zero (unbounded) comes
// back as zero, not as absent, and the reload comes back as written.
func TestReloadFiguresRoundTrip(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	p := New(db)
	p.Slug, p.Name, p.Category, p.Price, p.Currency = "ultra", "Ultra", "personal", 19500, currency.USD
	p.Limits = &Limits{Agents: n(10), ReloadsPerDay: n(0), Reload: &Reload{ThresholdCents: 1000, AmountCents: 5000}}
	if err := p.Create(); err != nil {
		t.Fatalf("create: %v", err)
	}

	got := New(db)
	if ok, err := got.Query().Filter("Slug=", "ultra").Get(); err != nil || !ok {
		t.Fatalf("ultra not read back: ok=%v err=%v", ok, err)
	}
	l := got.Limits
	if l == nil || l.ReloadsPerDay == nil || *l.ReloadsPerDay != 0 {
		t.Fatalf("reloadsPerDay read back as %+v, want a present 0", l)
	}
	if l.Reload == nil || *l.Reload != (Reload{ThresholdCents: 1000, AmountCents: 5000}) {
		t.Fatalf("reload read back as %+v, want $50 below $10", l.Reload)
	}
	if !limitsEqual(p.Limits, l) {
		t.Fatal("the limits read back differ from the limits written")
	}
}

// The reload is compared like every other limit, so a plan whose only change is
// its reload is reconciled rather than read as already published.
func TestAReloadChangeIsCompared(t *testing.T) {
	a := &Limits{Reload: &Reload{ThresholdCents: 1000, AmountCents: 5000}}
	for _, b := range []*Limits{
		{},
		{Reload: &Reload{ThresholdCents: 1000, AmountCents: 2500}},
		{Reload: &Reload{ThresholdCents: 500, AmountCents: 5000}},
	} {
		if limitsEqual(a, b) {
			t.Errorf("limitsEqual(%+v, %+v) = true", a.Reload, b.Reload)
		}
	}
	if !limitsEqual(a, &Limits{Reload: &Reload{ThresholdCents: 1000, AmountCents: 5000}}) {
		t.Error("two equal reloads compare unequal")
	}
}
