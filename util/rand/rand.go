package rand

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	mathrand "math/rand"
	"strings"
	"time"
)

// Returns short, url-friendly Id
func ShortPassword() string {
	size := 16

	rb := make([]byte, size)
	if _, err := rand.Read(rb); err != nil {
		fmt.Printf("Failed to genrate random characters: %v", err)
	}

	return strings.Trim(base64.URLEncoding.EncodeToString(rb), "=")
}

// Returns short, url-friendly Id
func ShortId() string {
	size := 8

	rb := make([]byte, size)
	if _, err := rand.Read(rb); err != nil {
		fmt.Printf("Failed to genrate random characters: %v", err)
	}

	return strings.Trim(base64.URLEncoding.EncodeToString(rb), "=")
}

func SecretKey() string {
	// 75% of 256 bytes
	size := 192

	rb := make([]byte, size)
	if _, err := rand.Read(rb); err != nil {
		fmt.Printf("Failed to genrate random characters: %v", err)
	}

	return strings.Trim(base64.URLEncoding.EncodeToString(rb), "=")
}

// String returns exactly n characters drawn uniformly from alphabet.
//
// It is the primitive to reach for when the output has a shape someone will
// read back or type in — a referral code, a coupon — rather than base64 with
// the awkward characters filtered out afterwards. Filtering is what made
// referral codes "eight characters, unless the draw happened to contain four
// of '-' and '_', in which case seven or fewer".
//
// Rejection sampling keeps the draw uniform. Folding a byte modulo the
// alphabet size would favour its first 256 % len(alphabet) characters, and a
// code with a lopsided alphabet is a code with less entropy than its length
// advertises.
func String(n int, alphabet string) string {
	if n <= 0 || len(alphabet) == 0 {
		return ""
	}

	// The largest multiple of len(alphabet) representable in a byte. Draws at
	// or above it are discarded rather than folded.
	limit := 256 - 256%len(alphabet)

	out := make([]byte, 0, n)
	buf := make([]byte, n)
	for len(out) < n {
		// crypto/rand.Read is documented never to fail; it crashes the program
		// itself if the system source is unavailable, which is the right
		// outcome here — a code from a broken source must never be issued.
		rand.Read(buf)

		for _, b := range buf {
			if int(b) >= limit {
				continue
			}
			out = append(out, alphabet[int(b)%len(alphabet)])
			if len(out) == n {
				break
			}
		}
	}

	return string(out)
}

func Int() int {
	return mathrand.Int()
}

func Int32() int32 {
	return mathrand.Int31()
}

func Int64() int64 {
	return mathrand.Int63()
}

func init() {
	mathrand.Seed(time.Now().UTC().UnixNano())
}
