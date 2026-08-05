package bitcoin

import (
	"encoding/json"
	"testing"

	"github.com/hanzoai/money"
)

// TestOutputValueConvertsToExactSatoshi pins the UTXO value Bitcoin Core reports.
//
// The field was float64 and the caller did int64(value * 100000000). Most BTC
// amounts have no exact binary form, so 1.15 decoded to 1.14999999… and the
// multiply truncated it to 114999999 — a satoshi short. That value feeds
// totalChange when building a real transaction, so the error lands in the
// change output and the fee.
//
// A satoshi is the minor unit of BTC, so money.BTC carries the right scale.
func TestOutputValueConvertsToExactSatoshi(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string // exactly as bitcoind puts it on the wire
		want  int64
	}{
		{"the case the float got wrong", "1.15", 115000000},
		{"another the float got wrong", "0.29", 29000000},
		{"and another", "21.4", 2140000000},
		{"already exact stays exact", "0.1", 10000000},
		{"one satoshi", "0.00000001", 1},
		{"whole coins", "2", 200000000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out GetRawTransactionResponseResultOutput
			if err := json.Unmarshal([]byte(`{"n":0,"value":`+tc.value+`}`), &out); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			got, err := money.ParseMinor(out.Value.String(), money.BTC)
			if err != nil {
				t.Fatalf("ParseMinor(%q): %v", out.Value, err)
			}
			if got != tc.want {
				t.Errorf("value %s = %d sat, want %d", tc.value, got, tc.want)
			}
		})
	}
}
