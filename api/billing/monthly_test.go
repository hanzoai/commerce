package billing

import (
	"testing"

	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/types/currency"
	types "github.com/hanzoai/commerce/types"
)

// A row stating no annual price — no AnnualTotal, no PriceAnnual, no legacy year —
// is sold by the month only: a yearly purchase is refused rather than priced at a
// figure no row states. The legacy rungs keep the year they were sold at.
func TestAMonthlyOnlyRowIsNotSoldAnnually(t *testing.T) {
	monthly := &plan.Plan{Slug: "go", Price: currency.Cents(800), Interval: types.Monthly, IntervalCount: 1}
	if at, err := planAtInterval(monthly, types.Yearly); err == nil {
		t.Fatalf("a monthly-only row was sold annually at %d cents", at.Price)
	} else if err.Error() != `plan "go" is not sold annually` {
		t.Fatalf("refusal = %q, want the plan named as not sold annually", err)
	}

	dev := &plan.Plan{Slug: "dev", Price: currency.Cents(2000), Interval: types.Monthly, IntervalCount: 1}
	year, err := planAtInterval(dev, types.Yearly)
	if err != nil {
		t.Fatalf("dev by the year: %v", err)
	}
	if year.Price != 20000 || year.Interval != types.Yearly {
		t.Fatalf("dev by the year is %d/%q, want its stated 20000/%q", year.Price, year.Interval, types.Yearly)
	}
}
