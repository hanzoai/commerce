// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package x402

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/luxfi/crypto"
)

// EIP-3009 TransferWithAuthorization, signed as EIP-712 typed data under the
// TOKEN's own domain — USDC's FiatTokenV2:
//
//	name              the token's name()    ("USD Coin" on Base, "USDC" on Base Sepolia)
//	version           the token's version() ("2")
//	chainId           the network's EVM chain id
//	verifyingContract the token contract
//
// The token checks exactly this digest in transferWithAuthorization, so a
// signature that recovers here under the token's domain is one the token
// accepts. This is the ONE EIP-3009 implementation in commerce: the x402
// facilitator and the USDC top-up rail (billing/gateway) both verify through
// it. The encoding is pinned in tests against digests and signatures that
// eth-account and viem produced independently.

// Domain is an EIP-712 domain, in the shape viem and ethers sign with.
type Domain struct {
	Name              string `json:"name"`
	Version           string `json:"version"`
	ChainID           int64  `json:"chainId"`
	VerifyingContract string `json:"verifyingContract"`
}

// Field is one member of an EIP-712 struct type.
type Field struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// types is the EIP-712 type table a payer signs with. The digest below is
// computed FROM it, so the types a payer is handed (Types) and the types a
// signature is verified against are one definition.
var types = map[string][]Field{
	"EIP712Domain": {
		{"name", "string"},
		{"version", "string"},
		{"chainId", "uint256"},
		{"verifyingContract", "address"},
	},
	"TransferWithAuthorization": {
		{"from", "address"},
		{"to", "address"},
		{"value", "uint256"},
		{"validAfter", "uint256"},
		{"validBefore", "uint256"},
		{"nonce", "bytes32"},
	},
}

// Types is a copy of the type table, to hand a payer. A copy, because the
// table is what every digest is computed from and a caller holding the
// original could change what this process verifies.
func Types() map[string][]Field {
	out := make(map[string][]Field, len(types))
	for k, v := range types {
		out[k] = append([]Field(nil), v...)
	}
	return out
}

// Primary is the type a payer signs.
const Primary = "TransferWithAuthorization"

// Authorization is an EIP-3009 TransferWithAuthorization as it travels: every
// field a string, exactly as the payer signed it.
type Authorization struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Value       string `json:"value"`
	ValidAfter  string `json:"validAfter"`
	ValidBefore string `json:"validBefore"`
	Nonce       string `json:"nonce"`
}

// Parsed is an Authorization whose every field has been read strictly. A
// field that does not parse is an error, never a zero: an address that failed
// to decode and hashed as zero would verify against the zero address.
type Parsed struct {
	From        [20]byte
	To          [20]byte
	Value       *big.Int
	ValidAfter  uint64
	ValidBefore uint64
	Nonce       [32]byte
}

// Parse reads a strictly: addresses are 0x + 40 hex, value is decimal digits
// below 2^256, the window is decimal unix seconds, and the nonce is exactly 32
// bytes (0x + 64 hex). Exactly 32, not "up to": a shorter nonce left-pads to
// the same bytes32, so "0x01" and "0x0001" would be two spellings of one
// signature and two keys for one payment.
func (a Authorization) Parse() (Parsed, error) {
	var p Parsed
	var err error
	if p.From, err = Address(a.From); err != nil {
		return p, fmt.Errorf("from: %w", err)
	}
	if p.To, err = Address(a.To); err != nil {
		return p, fmt.Errorf("to: %w", err)
	}
	if p.Value, err = amount(a.Value); err != nil {
		return p, fmt.Errorf("value: %w", err)
	}
	if p.ValidAfter, err = seconds(a.ValidAfter); err != nil {
		return p, fmt.Errorf("validAfter: %w", err)
	}
	if p.ValidBefore, err = seconds(a.ValidBefore); err != nil {
		return p, fmt.Errorf("validBefore: %w", err)
	}
	if p.Nonce, err = Nonce(a.Nonce); err != nil {
		return p, fmt.Errorf("nonce: %w", err)
	}
	return p, nil
}

// Separator is the domain's EIP-712 separator: what a token's
// DOMAIN_SEPARATOR() answers when d is its domain. A malformed contract or a
// non-positive chain id is an error, never a separator over zeroes.
func (d Domain) Separator() ([32]byte, error) {
	var out [32]byte
	contract, err := Address(d.VerifyingContract)
	if err != nil {
		return out, fmt.Errorf("verifyingContract: %w", err)
	}
	if d.ChainID <= 0 {
		return out, fmt.Errorf("chainId %d", d.ChainID)
	}
	var dom []byte
	dom = append(dom, typeHash("EIP712Domain")...)
	dom = append(dom, crypto.Keccak256([]byte(d.Name))...)
	dom = append(dom, crypto.Keccak256([]byte(d.Version))...)
	dom = append(dom, word(new(big.Int).SetInt64(d.ChainID))...)
	dom = append(dom, pad(contract[:])...)
	copy(out[:], crypto.Keccak256(dom))
	return out, nil
}

// Digest is the EIP-712 hash a payer signs for p under d.
func (d Domain) Digest(p Parsed) ([32]byte, error) {
	sep, err := d.Separator()
	if err != nil {
		return [32]byte{}, err
	}

	var msg []byte
	msg = append(msg, typeHash(Primary)...)
	msg = append(msg, pad(p.From[:])...)
	msg = append(msg, pad(p.To[:])...)
	msg = append(msg, word(p.Value)...)
	msg = append(msg, word(new(big.Int).SetUint64(p.ValidAfter))...)
	msg = append(msg, word(new(big.Int).SetUint64(p.ValidBefore))...)
	msg = append(msg, p.Nonce[:]...)

	var out [32]byte
	copy(out[:], crypto.Keccak256([]byte{0x19, 0x01}, sep[:], crypto.Keccak256(msg)))
	return out, nil
}

// encoding is the EIP-712 encodeType of a struct with no struct members, read
// off Types: "TransferWithAuthorization(address from,address to,…)".
func encoding(name string) string {
	parts := make([]string, 0, len(types[name]))
	for _, f := range types[name] {
		parts = append(parts, f.Type+" "+f.Name)
	}
	return name + "(" + strings.Join(parts, ",") + ")"
}

func typeHash(name string) []byte { return crypto.Keccak256([]byte(encoding(name))) }

// Recover returns the address that produced sig over digest.
//
// sig is 65 bytes r‖s‖v in hex, v in {0, 1, 27, 28}. A high-s signature is
// refused: (r, n−s) is the same signature in the other half of the curve, and
// USDC's signature check (EIP-2) refuses it, so accepting it here would accept
// a payment the token cannot settle.
func Recover(digest [32]byte, sig string) ([20]byte, error) {
	var out [20]byte
	b, err := hexBytes(sig)
	if err != nil {
		return out, err
	}
	if len(b) != 65 {
		return out, fmt.Errorf("signature is %d bytes, want 65", len(b))
	}
	v := b[64]
	if v >= 27 {
		v -= 27
	}
	r, s := new(big.Int).SetBytes(b[:32]), new(big.Int).SetBytes(b[32:64])
	if !crypto.ValidateSignatureValues(v, r, s, true) {
		return out, errors.New("signature values out of range (r, s, v, or high s)")
	}
	norm := make([]byte, 65)
	copy(norm, b)
	norm[64] = v
	pub, err := crypto.SigToPub(digest[:], norm)
	if err != nil {
		return out, fmt.Errorf("recover: %w", err)
	}
	copy(out[:], crypto.PubkeyToAddress(*pub).Bytes())
	return out, nil
}

// Canonical is sig's one spelling: lowercase 0x hex, v as 27 or 28 — the form
// transferWithAuthorization's v argument takes.
// It assumes Recover has already accepted sig.
func Canonical(sig string) (string, error) {
	b, err := hexBytes(sig)
	if err != nil {
		return "", err
	}
	if len(b) != 65 {
		return "", fmt.Errorf("signature is %d bytes, want 65", len(b))
	}
	if b[64] < 27 {
		b[64] += 27
	}
	return Hex(b), nil
}

// Address reads a 0x-prefixed 20-byte hex address, any case. A checksum is
// not required, so it is not checked: EIP-55 protects a human typing an
// address, and none is typed here.
func Address(s string) ([20]byte, error) {
	var out [20]byte
	b, err := hexBytes(s)
	if err != nil {
		return out, err
	}
	if len(b) != 20 {
		return out, fmt.Errorf("address %q is %d bytes, want 20", s, len(b))
	}
	copy(out[:], b)
	return out, nil
}

// Nonce reads a 0x-prefixed 32-byte hex nonce, exactly 64 hex digits.
func Nonce(s string) ([32]byte, error) {
	var out [32]byte
	b, err := hexBytes(s)
	if err != nil {
		return out, err
	}
	if len(b) != 32 {
		return out, fmt.Errorf("nonce is %d bytes, want exactly 32", len(b))
	}
	copy(out[:], b)
	return out, nil
}

// Hex renders bytes as lowercase 0x hex: the ONE spelling stored and compared.
func Hex(b []byte) string { return "0x" + hex.EncodeToString(b) }

// hexBytes decodes 0x-prefixed hex. The prefix is required: a bare hex string
// is ambiguous with a decimal one in every field that carries both.
func hexBytes(s string) ([]byte, error) {
	if len(s) < 2 || s[0] != '0' || (s[1] != 'x' && s[1] != 'X') {
		return nil, fmt.Errorf("%q is not 0x-prefixed hex", s)
	}
	b, err := hex.DecodeString(s[2:])
	if err != nil {
		return nil, fmt.Errorf("%q is not hex: %w", s, err)
	}
	return b, nil
}

// maxWord is 2^256 − 1, the largest uint256.
var maxWord = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

// amount reads a uint256 in decimal: digits only, no sign, no leading zero.
func amount(s string) (*big.Int, error) {
	if !digits(s) || len(s) > 78 {
		return nil, fmt.Errorf("%q is not a decimal uint256", s)
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok || v.Cmp(maxWord) > 0 {
		return nil, fmt.Errorf("%q is not a uint256", s)
	}
	return v, nil
}

// seconds reads unix seconds in decimal. A uint256 past 2^64 is refused rather
// than read: no window this rail accepts reaches it.
func seconds(s string) (uint64, error) {
	if !digits(s) || len(s) > 20 {
		return 0, fmt.Errorf("%q is not decimal unix seconds", s)
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok || !v.IsUint64() {
		return 0, fmt.Errorf("%q is out of range", s)
	}
	return v.Uint64(), nil
}

// digits reports a canonical decimal: [0-9]+ with no leading zero but "0"
// itself, so one number has one spelling.
func digits(s string) bool {
	if s == "" || (len(s) > 1 && s[0] == '0') {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// pad left-pads b to a 32-byte word.
func pad(b []byte) []byte {
	out := make([]byte, 32)
	copy(out[32-len(b):], b)
	return out
}

// word is v as a 32-byte big-endian word. v is never negative and never past
// 2^256 here: amount and seconds refused anything else.
func word(v *big.Int) []byte {
	out := make([]byte, 32)
	v.FillBytes(out)
	return out
}
