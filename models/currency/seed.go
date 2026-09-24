package currency

import "github.com/hanzoai/commerce/datastore"

// seedRow is one reference currency: ISO-4217 code, display name, symbol, and
// minor-unit decimal places. Kept as a small curated table (not derived from
// models/types/currency) so the reference data is explicit and correct — the
// value type's Symbol() returns "" for a few codes (inr/try) we want populated.
type seedRow struct {
	Code    string
	Name    string
	Symbol  string
	Decimal int
}

// commonCurrencies is the set every store starts accepting. Ordered by
// prevalence. Zero-decimal currencies (jpy/krw) carry Decimal 0.
var commonCurrencies = []seedRow{
	{"usd", "US Dollar", "$", 2},
	{"eur", "Euro", "€", 2},
	{"gbp", "British Pound", "£", 2},
	{"cad", "Canadian Dollar", "$", 2},
	{"aud", "Australian Dollar", "$", 2},
	{"jpy", "Japanese Yen", "¥", 0},
	{"chf", "Swiss Franc", "CHF", 2},
	{"cny", "Chinese Yuan", "¥", 2},
	{"inr", "Indian Rupee", "₹", 2},
	{"nzd", "New Zealand Dollar", "$", 2},
	{"sgd", "Singapore Dollar", "$", 2},
	{"hkd", "Hong Kong Dollar", "$", 2},
	{"sek", "Swedish Krona", "kr", 2},
	{"nok", "Norwegian Krone", "kr", 2},
	{"dkk", "Danish Krone", "kr", 2},
	{"pln", "Polish Złoty", "zł", 2},
	{"mxn", "Mexican Peso", "$", 2},
	{"brl", "Brazilian Real", "R$", 2},
	{"zar", "South African Rand", "R", 2},
	{"aed", "UAE Dirham", "د.إ", 2},
	{"krw", "South Korean Won", "₩", 0},
	{"try", "Turkish Lira", "₺", 2},
}

// Seed inserts the common currencies into db (which must be the default-namespace
// reference store). IDEMPOTENT + NON-DESTRUCTIVE: a currency whose code already
// exists is left untouched (admin edits are never clobbered); only missing codes
// are created. Returns the number created.
func Seed(db *datastore.Datastore) (created int, err error) {
	for _, r := range commonCurrencies {
		existing := New(db)
		ok, qerr := existing.Query().Filter("Code=", r.Code).Get()
		if qerr != nil {
			return created, qerr
		}
		if ok {
			continue
		}
		c := New(db)
		c.Code = r.Code
		c.Name = r.Name
		c.Symbol = r.Symbol
		c.DecimalDigits = r.Decimal
		c.Normalize()
		if err := c.Create(); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// SeedIfEmpty seeds the reference currencies only when the table is empty — a
// cheap count query gates the per-row create, so it is safe to call on every
// bootstrap (mirrors catalogentry.SeedIfEmpty). Once any currency exists (seeded
// or admin-created) it never re-runs, so admin edits stay authoritative.
func SeedIfEmpty(db *datastore.Datastore) (created int, err error) {
	n, err := Query(db).Count()
	if err != nil {
		return 0, err
	}
	if n > 0 {
		return 0, nil
	}
	return Seed(db)
}
