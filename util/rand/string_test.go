package rand

import (
	"strings"
	"testing"
)

const alnum = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func TestStringLengthAndAlphabet(t *testing.T) {
	for _, n := range []int{1, 7, 8, 9, 64, 1000} {
		s := String(n, alnum)
		if len(s) != n {
			t.Fatalf("String(%d, alnum) = %q, length %d", n, s, len(s))
		}
		for _, r := range s {
			if !strings.ContainsRune(alnum, r) {
				t.Fatalf("String(%d, alnum) = %q, contains %q", n, s, r)
			}
		}
	}
}

func TestStringDegenerateInputs(t *testing.T) {
	if s := String(0, alnum); s != "" {
		t.Fatalf("String(0, alnum) = %q, want empty", s)
	}
	if s := String(-1, alnum); s != "" {
		t.Fatalf("String(-1, alnum) = %q, want empty", s)
	}
	if s := String(8, ""); s != "" {
		t.Fatalf("String(8, \"\") = %q, want empty", s)
	}
}

// The draw must be uniform, which is the reason String rejects the top of the
// byte range instead of folding it. 256 is not a multiple of 36, so folding
// would hand the first four characters of this alphabet an eighth more weight
// than the rest — entropy quietly below what the length advertises.
//
// Expect n/36 of each; the counts are ~5 standard deviations inside this
// tolerance when the draw is uniform, and well outside it when it is folded.
func TestStringIsUniform(t *testing.T) {
	const draws = 360000

	counts := make(map[rune]int, len(alnum))
	for _, r := range String(draws, alnum) {
		counts[r]++
	}

	if len(counts) != len(alnum) {
		t.Fatalf("saw %d distinct characters, want %d", len(counts), len(alnum))
	}

	expected := draws / len(alnum)
	tolerance := expected / 20 // 5%
	for _, r := range alnum {
		if delta := counts[r] - expected; delta > tolerance || delta < -tolerance {
			t.Errorf("%q drawn %d times, want %d ± %d", r, counts[r], expected, tolerance)
		}
	}
}
