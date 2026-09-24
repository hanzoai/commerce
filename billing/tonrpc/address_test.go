package tonrpc

import (
	"strings"
	"testing"
)

// Real mainnet identifiers. The USDT jetton master, in the three spellings the
// same account is published in.
const (
	usdtMasterEQ  = "EQCxE6mUtQJKFnGfaROTKOt1lZbDiiX1kCixRv7Nw2Id_sDs"
	usdtMasterUQ  = "UQCxE6mUtQJKFnGfaROTKOt1lZbDiiX1kCixRv7Nw2Id_p0p"
	usdtMasterRaw = "0:B113A994B5024A16719F69139328EB759596C38A25F59028B146FECDC3621DFE"
)

// THE test for this file. A TON account has several correct spellings, and if
// they did not parse to the same value a custody address minted in one form and
// reported by the index in another would never match — every deposit invisible,
// nothing in the logs, the rail looking healthy.
func TestParseAddress_AllSpellingsAreOneAccount(t *testing.T) {
	eq, err := ParseAddress(usdtMasterEQ)
	if err != nil {
		t.Fatalf("bounceable form: %v", err)
	}
	if got := eq.String(); got != usdtMasterRaw {
		t.Fatalf("EQ form canonicalised to %s, want %s", got, usdtMasterRaw)
	}
	raw, err := ParseAddress(usdtMasterRaw)
	if err != nil {
		t.Fatalf("raw form: %v", err)
	}
	if raw != eq {
		t.Fatal("the raw and bounceable spellings of one account do not compare equal")
	}
	uq, err := ParseAddress(usdtMasterUQ)
	if err != nil {
		t.Fatalf("non-bounceable form: %v", err)
	}
	if uq != eq {
		t.Fatal("the bounceable and non-bounceable spellings of one account do not compare equal")
	}
	if _, err := ParseAddress("  " + usdtMasterEQ + "  "); err != nil {
		t.Fatalf("surrounding whitespace was not trimmed: %v", err)
	}
}

// The masterchain is workchain −1, and its byte is 0xFF. Read unsigned it
// becomes workchain 255, an account that does not exist.
func TestParseAddress_WorkchainIsSigned(t *testing.T) {
	a, err := ParseAddress("-1:" + strings.Repeat("A", 64))
	if err != nil {
		t.Fatal(err)
	}
	if a.Workchain != -1 {
		t.Fatalf("workchain = %d, want -1", a.Workchain)
	}
	if !strings.HasPrefix(a.String(), "-1:") {
		t.Fatalf("rendered as %s", a.String())
	}
}

// The user-friendly form carries a CRC-16. It is what turns a mistyped custody
// address into a boot failure instead of a deposit nobody ever sees.
func TestParseAddress_Refusals(t *testing.T) {
	for _, tc := range []struct{ name, in, wantIn string }{
		{"empty", "", "empty"},
		{"one character changed", "EQCxE6mUtQJKFnGfaROTKOt1lZbDiiX1kCixRv7Nw2Id_sDt", "CRC"},
		{"two characters transposed", "EQCxE6mUtQJKFnGfaROTKOt1lZbDiiX1kCixRv7Nw2Id_sSD", "CRC"},
		{"truncated", "EQCxE6mUtQJKFnGfaROTKOt1lZbDiiX1kCixRv7Nw2Id_sD", "not a base64 TON address"},
		{"a Solana mint", "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "bytes"},
		{"an EVM address", "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", "not a base64 TON address"},
		{"raw with a short hash", "0:B113A994", "<workchain>:<64 hex>"},
		{"raw with a non-hex hash", "0:" + strings.Repeat("Z", 64), "<workchain>:<64 hex>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseAddress(tc.in)
			if err == nil {
				t.Fatalf("ParseAddress(%q) = %s, want a refusal", tc.in, got)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Fatalf("error %q does not mention %q", err, tc.wantIn)
			}
		})
	}
}

// A testnet address is a DIFFERENT network's account spelled almost identically.
// Crediting mainnet balance against one is worth failing loudly on.
func TestParseAddress_RefusesTestnet(t *testing.T) {
	// The same account with the test-only flag (0x80) set on its tag, CRC
	// recomputed — i.e. a genuinely valid testnet address, not a corrupted one.
	body := make([]byte, 36)
	eq, err := ParseAddress(usdtMasterEQ)
	if err != nil {
		t.Fatal(err)
	}
	body[0] = tagBounceable | tagTestOnly
	body[1] = 0x00
	copy(body[2:34], eq.Hash[:])
	crc := crc16XModem(body[:34])
	body[34], body[35] = byte(crc>>8), byte(crc)

	testnet := base64URL(body)
	if _, err := ParseAddress(testnet); err == nil {
		t.Fatalf("accepted the testnet address %s", testnet)
	} else if !strings.Contains(err.Error(), "TESTNET") {
		t.Fatalf("error %q does not say it is a testnet address", err)
	}
}

// A transaction id goes into the dedup key, and the dedup key is what makes a
// credit happen exactly once. It must therefore be a function of the EVENT and
// not of how an endpoint chose to print it: this index prints hashes in base64,
// others print hex, and a re-scan through either must land on one ledger row.
func TestDecodeHash_IsCanonicalWhicheverWayItArrives(t *testing.T) {
	const b64 = "1YabVkukw02sgxSTXJDesmBJnzQn0qqd1yBw1KNosgw="
	const wantHex = "d5869b564ba4c34dac8314935c90deb260499f3427d2aa9dd72070d4a368b20c"

	got, err := decodeHash(b64)
	if err != nil {
		t.Fatal(err)
	}
	if got != wantHex {
		t.Fatalf("decodeHash(base64) = %s, want %s", got, wantHex)
	}
	fromHex, err := decodeHash(strings.ToUpper(wantHex))
	if err != nil {
		t.Fatal(err)
	}
	if fromHex != got {
		t.Fatalf("the same hash rendered as uppercase hex produced %s, not %s — two dedup keys for one deposit", fromHex, got)
	}
	for _, bad := range []string{"", "not base64!!", "AAAA", strings.Repeat("a", 62)} {
		if _, err := decodeHash(bad); err == nil {
			t.Fatalf("decodeHash(%q) was accepted", bad)
		}
	}
}

// base64URL is the encoder TON publishes addresses with, spelled out here so
// the test above builds its fixture the same way ParseAddress reads it.
func base64URL(b []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	var sb strings.Builder
	for i := 0; i < len(b); i += 3 {
		n := uint32(b[i]) << 16
		if i+1 < len(b) {
			n |= uint32(b[i+1]) << 8
		}
		if i+2 < len(b) {
			n |= uint32(b[i+2])
		}
		sb.WriteByte(alphabet[n>>18&0x3f])
		sb.WriteByte(alphabet[n>>12&0x3f])
		sb.WriteByte(alphabet[n>>6&0x3f])
		sb.WriteByte(alphabet[n&0x3f])
	}
	return sb.String()
}
