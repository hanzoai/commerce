package currency

import "testing"

// The float implementation truncated: 19.99 has no exact binary form, so
// int64(f*100) was 1998. These are real price points; each was undercharging
// by a cent on money already captured.
func TestCentsFromStringDoesNotLoseACent(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Cents
	}{
		{"19.99", 1999},
		{"9.95", 995},
		{"1.15", 115},
		{"0.29", 29},
		{"20.15", 2015},
		{"0", 0},
		{"0.00", 0},
		{"100", 10000},
		{"1234.56", 123456},
		{" 19.99 ", 1999},
		{"-5.00", -500}, // a refund must not round toward zero
	} {
		got, err := CentsFromString(tc.in)
		if err != nil {
			t.Errorf("CentsFromString(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("CentsFromString(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// An unparseable amount used to become zero — a free transaction reporting
// success. It must be an error the caller has to handle.
func TestCentsFromStringRefusesGarbage(t *testing.T) {
	for _, in := range []string{"", "abc", "1,234.56", "$5.00", "1.2.3"} {
		if got, err := CentsFromString(in); err == nil {
			t.Errorf("CentsFromString(%q) = %d with no error; a bad amount must not become a number", in, got)
		}
	}
}
