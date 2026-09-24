// Package xrplrpc reads XRP Ledger deposits for ONE issued currency over the
// XRPL JSON-RPC API. It is the XRPL half of the crypto top-up rail's read side,
// beside billing/husdindex (EVM) and billing/solanarpc (Solana), and it
// satisfies depositwatch.Reader like both of them.
//
// TWO THINGS ABOUT XRPL ARE GENUINELY DIFFERENT, and neither is cosmetic:
//
//  1. THE DEPOSIT MODEL IS POOLED. A funded XRPL account costs a NON-REFUNDABLE
//     base reserve in XRP, so minting a fresh address per payer strands that
//     reserve on every single deposit, forever. The production model — the one
//     every exchange on this ledger uses — is ONE account plus a per-deposit
//     DESTINATION TAG. That inverts the address→payer mapping every other chain
//     in this rail relies on: an XRPL address identifies the CUSTODIAN, and only
//     (address, destinationTag) identifies the payer. See depositwatch.Asset's
//     Identity and Pooled.
//
//  2. AMOUNTS ARE DECIMAL STRINGS, NOT INTEGER BASE UNITS. An issued currency
//     has no decimals() to read, because it has no base unit at all — the ledger
//     states a number like "1234.56". See amount.go, which is where that is
//     turned into something the rail's cent arithmetic can consume, and which is
//     the file to be paranoid about.
package xrplrpc

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// --- accounts ---

// Account is a decoded XRPL classic account id (the 20 bytes behind an
// r-address). Parsing to bytes rather than trusting the string is what makes a
// mistyped custody address fail at boot instead of at the first deposit: the
// base58 encoding carries a checksum, and a typo does not survive it.
type Account [20]byte

// rippleAlphabet is XRPL's base58 alphabet. It is NOT Bitcoin's — the same 58
// characters in a different order — so decoding an XRPL address with a Bitcoin
// decoder yields 20 plausible bytes that are the wrong account.
const rippleAlphabet = "rpshnaf39wBUDNEGHJKLM4PQRST7VWXYZ2bcdeCg65jkm8oFqi1tuvAxyz"

// accountPrefix is the version byte of a classic account address; it is what
// makes such an address start with 'r'.
const accountPrefix = 0x00

// ParseAccount decodes and CHECKSUMS an r-address.
//
// Refusing anything that is not exactly a 20-byte classic account id also
// refuses the two things most likely to be pasted into an XRPL slot by mistake:
// an X-address (which starts with 'X' and encodes a destination tag INSIDE the
// address — this rail carries the tag in its own field, so an address that
// hides one would silently route a deposit to the wrong intent), and a seed or
// public key, which have their own version bytes.
func ParseAccount(s string) (Account, error) {
	var a Account
	s = strings.TrimSpace(s)
	if s == "" {
		return a, fmt.Errorf("xrplrpc: empty account address")
	}
	raw, err := base58Decode(s)
	if err != nil {
		return a, fmt.Errorf("xrplrpc: %q is not a base58 XRPL address: %w", s, err)
	}
	if len(raw) != 25 {
		return a, fmt.Errorf("xrplrpc: %q decodes to %d bytes, not the 25 of a classic account address", s, len(raw))
	}
	if raw[0] != accountPrefix {
		return a, fmt.Errorf("xrplrpc: %q has version byte 0x%02x, not the 0x00 of a classic account address — an X-address, seed or key is not a deposit destination", s, raw[0])
	}
	first := sha256.Sum256(raw[:21])
	second := sha256.Sum256(first[:])
	for i := 0; i < 4; i++ {
		if second[i] != raw[21+i] {
			return a, fmt.Errorf("xrplrpc: %q fails its checksum — it is a mistyped address, not an account", s)
		}
	}
	copy(a[:], raw[1:21])
	return a, nil
}

// base58Decode decodes XRPL base58 into bytes, preserving leading zero bytes.
func base58Decode(s string) ([]byte, error) {
	n := new(big.Int)
	radix := big.NewInt(58)
	for _, r := range s {
		i := strings.IndexRune(rippleAlphabet, r)
		if i < 0 {
			return nil, fmt.Errorf("character %q is not in the XRPL base58 alphabet", r)
		}
		n.Add(n.Mul(n, radix), big.NewInt(int64(i)))
	}
	body := n.Bytes()
	// Each leading 'r' (alphabet index 0) is one leading ZERO byte, which the
	// big.Int has dropped. Restoring them is what makes the length check above
	// mean anything.
	zeros := 0
	for _, r := range s {
		if r != rune(rippleAlphabet[0]) {
			break
		}
		zeros++
	}
	out := make([]byte, zeros+len(body))
	copy(out[zeros:], body)
	return out, nil
}

// --- currencies ---

// Currency is a 160-bit XRPL currency code.
//
// It is kept as BYTES and never as a string because the ledger renders the same
// code two ways: a 3-character code comes back as "USD", while everything else
// comes back as 40 hex characters. Comparing the rendered forms would make
// "USD" and its hex spelling two different tokens; comparing the 160 bits makes
// them the one token they are.
type Currency [20]byte

// ParseCurrency reads a currency code in any of the three forms the ledger and
// its tooling use, and produces the 160 bits all three name.
//
//	"USD"                        the STANDARD format: three ASCII characters at
//	                             bytes 12..14, everything else zero.
//	"524C5553440000…"            the raw 40-hex code, which is how the ledger
//	                             renders anything that is not the standard
//	                             format.
//	"RLUSD"                      a ticker of 1–20 characters other than 3, which
//	                             is the same 160 bits written as ASCII
//	                             left-aligned and NUL padded — the convention
//	                             every ticker longer than three characters uses,
//	                             and verifiably what RLUSD's on-ledger code is.
//
// The last form is a convenience for configuration, and it is SAFE to guess at
// because a wrong guess cannot mis-credit: the resulting 160 bits either match
// what the issuer actually issues, in which case they are the token, or they do
// not, in which case Client.Symbol finds nothing and the asset is refused.
func ParseCurrency(s string) (Currency, error) {
	var c Currency
	s = strings.TrimSpace(s)
	switch {
	case len(s) == 3:
		for i := 0; i < 3; i++ {
			if s[i] < 0x21 || s[i] > 0x7e {
				return c, fmt.Errorf("xrplrpc: currency %q is not printable ASCII", s)
			}
			c[12+i] = s[i]
		}
	case len(s) == 40 && isHex(s):
		b, err := hex.DecodeString(s)
		if err != nil {
			return c, fmt.Errorf("xrplrpc: currency %q is not 40 hex characters: %w", s, err)
		}
		copy(c[:], b)
	case len(s) >= 1 && len(s) <= 20:
		for i := 0; i < len(s); i++ {
			if s[i] < 0x21 || s[i] > 0x7e {
				return c, fmt.Errorf("xrplrpc: currency %q is not printable ASCII", s)
			}
			c[i] = s[i]
		}
		if c[0] == 0x00 {
			return c, fmt.Errorf("xrplrpc: currency %q cannot be written as a non-standard code", s)
		}
	default:
		return c, fmt.Errorf("xrplrpc: currency %q is neither a 3-character code, a ticker of up to 20 characters, nor a 40-character hex code", s)
	}
	if c == (Currency{}) {
		return c, fmt.Errorf("xrplrpc: the all-zero currency code is XRP, which this rail does not credit")
	}
	if c.Symbol() == "XRP" {
		return c, fmt.Errorf("xrplrpc: XRP is the native coin, has no issuer, and cannot be credited at a dollar peg")
	}
	return c, nil
}

// String renders the canonical 40-character hex form.
func (c Currency) String() string { return strings.ToUpper(hex.EncodeToString(c[:])) }

// Symbol decodes the human ticker a currency code carries, or "" when it
// carries none.
//
// Two encodings exist and both are in production use on this ledger:
//
//	standard     byte 0 is 0x00 and the ticker is the 3 ASCII bytes at 12..14
//	             ("USD" → 0000…005553440000000000)
//	non-standard the 20 bytes are the ticker in ASCII, left-aligned and NUL
//	             padded, which is how every ticker longer than 3 characters is
//	             written ("RLUSD" → 524C555344000…0)
//
// Anything else is opaque 160-bit binary. It gets "" — an unnameable token on a
// rail that credits at a fixed dollar peg is exactly the case to fail closed
// on, and Client.Symbol turns "" into a refusal.
func (c Currency) Symbol() string {
	if c[0] == 0x00 {
		var std Currency
		copy(std[12:15], c[12:15])
		if c != std {
			return "" // 0x00-prefixed but not the standard layout
		}
		return printableASCII(c[12:15])
	}
	return printableASCII(c[:])
}

func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f' || ch >= 'A' && ch <= 'F') {
			return false
		}
	}
	return len(s) > 0
}

// printableASCII returns b as a ticker, or "" if b is not NUL-padded printable
// ASCII.
func printableASCII(b []byte) string {
	end := len(b)
	for end > 0 && b[end-1] == 0x00 {
		end--
	}
	if end == 0 {
		return ""
	}
	for _, ch := range b[:end] {
		if ch < 0x21 || ch > 0x7e {
			return ""
		}
	}
	return string(b[:end])
}

// --- the configured asset: currency + issuer ---

// Issued is the pair that IS a token on XRPL. Neither half identifies one
// alone: an issuer issues many currencies, and a currency code is just a name
// that anybody may issue. A deposit of "USDC" from an issuer we did not
// configure is a different token — very possibly a worthless one minted to look
// like ours — so both halves are compared on every transfer.
type Issued struct {
	Currency Currency
	Issuer   string // the issuer's r-address, canonical (checksummed at parse)
}

// ParseIssued reads the configured token, written <CURRENCY>.<ISSUER> — the
// form XRPL tooling has settled on, e.g.
// RLUSD.rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De.
func ParseIssued(s string) (Issued, error) {
	var is Issued
	s = strings.TrimSpace(s)
	dot := strings.LastIndexByte(s, '.')
	if dot <= 0 || dot == len(s)-1 {
		return is, fmt.Errorf("xrplrpc: %q is not <CURRENCY>.<ISSUER>", s)
	}
	cur, err := ParseCurrency(s[:dot])
	if err != nil {
		return is, err
	}
	issuer := strings.TrimSpace(s[dot+1:])
	if _, err := ParseAccount(issuer); err != nil {
		return is, err
	}
	return Issued{Currency: cur, Issuer: issuer}, nil
}

// String renders the token back in its configured form.
func (i Issued) String() string { return i.Currency.String() + "." + i.Issuer }
