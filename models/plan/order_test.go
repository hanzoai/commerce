package plan

import "testing"

// TestLimitsEqual_TheOrderFigures: the seed compares a row's order figures, so an
// admin-edited row that differs from the catalog only in them stays the admin's
// and is never handed back for a later boot to clear.
func TestLimitsEqual_TheOrderFigures(t *testing.T) {
	n := func(v int) *int { return &v }
	base := func() *Limits {
		return &Limits{MinSeats: n(20), MaxMembers: n(500), SpendLimits: []int{20, 50}, SpendLimit: n(20), BalanceMonths: n(1), BalanceCeiling: n(50000)}
	}
	if !limitsEqual(base(), base()) {
		t.Fatal("equal order figures read as different")
	}
	for name, edit := range map[string]func(*Limits){
		"spendLimits":    func(l *Limits) { l.SpendLimits = []int{20, 100} },
		"spendLimit":     func(l *Limits) { l.SpendLimit = n(50) },
		"balanceMonths":  func(l *Limits) { l.BalanceMonths = n(2) },
		"balanceCeiling": func(l *Limits) { l.BalanceCeiling = nil },
	} {
		l := base()
		edit(l)
		if limitsEqual(base(), l) {
			t.Fatalf("%s differs and read as equal", name)
		}
	}
	if (&Limits{MinSeats: n(2)}).Ordered() || !(&Limits{BalanceCeiling: n(1)}).Ordered() {
		t.Fatal("Ordered reads a seat floor as an order, or misses a ceiling")
	}
	var none *Limits
	if none.Ordered() {
		t.Fatal("no limits read as an order")
	}
}
