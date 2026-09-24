// Package solanarpc is the Solana half of the crypto deposit rail's chain
// READS: block height, SPL mint metadata, and the token transfers that landed
// in a set of watched addresses.
//
// It is to Solana what billing/husdindex's client is to the EVM — a minimal
// JSON-RPC read client with no SDK, no cgo and no signing — and it satisfies
// the same billing/depositwatch.Reader interface, so every crediting DECISION
// stays in depositwatch where it is proven against fakes.
//
// Three Solana facts shape this package, and each one is a place the EVM path
// has no equivalent:
//
//   - An SPL transfer does not land at the address the customer is shown. It
//     lands in the Associated Token Account derived from (owner, token program,
//     mint). The owner never appears in the account list of a plain transfer, so
//     watching it would see nothing at all; this package derives and watches the
//     ATA, and reports the OWNER back so the caller matches what it minted.
//   - There is no eth_getLogs. Deposits are found per-address with
//     getSignaturesForAddress, which makes a pass cost one RPC call per watched
//     address rather than one per chunk of a hundred. That is a property of the
//     chain, stated here rather than discovered in a bill.
//   - Amounts come from the transaction's own token BALANCE records, not from
//     decoding a transfer instruction. The balance delta is what actually
//     arrived; an instruction's amount field is what was sent, and under
//     Token-2022 transfer fees those are different numbers.
package solanarpc

import (
	"crypto/sha256"
	"fmt"

	"filippo.io/edwards25519"
	"github.com/mr-tron/base58"
)

// PublicKey is a 32-byte Solana account address.
//
// It is a fixed-size array rather than a string because every use of an address
// here is either a seed for a derivation or an exact comparison, and both are
// wrong on a string: base58 is CASE-SENSITIVE, so the case-folding that makes
// EVM hex safe to compare would silently merge distinct accounts.
type PublicKey [32]byte

// ParsePublicKey decodes a base58 address. Anything that is not exactly 32
// bytes is refused: base58 has no length in its encoding, so a truncated or
// pasted-twice address decodes happily into the wrong number of bytes.
func ParsePublicKey(s string) (PublicKey, error) {
	b, err := base58.Decode(s)
	if err != nil {
		return PublicKey{}, fmt.Errorf("solanarpc: %q is not base58: %w", s, err)
	}
	if len(b) != 32 {
		return PublicKey{}, fmt.Errorf("solanarpc: %q decodes to %d bytes, not a 32-byte address", s, len(b))
	}
	var k PublicKey
	copy(k[:], b)
	return k, nil
}

// MustPublicKey parses a compile-time constant address. It panics, which is
// correct only for the program ids below — a typo in one of those is a bug in
// this file, not a runtime condition.
func MustPublicKey(s string) PublicKey {
	k, err := ParsePublicKey(s)
	if err != nil {
		panic(err)
	}
	return k
}

func (k PublicKey) String() string { return base58.Encode(k[:]) }

// The program ids this package derives against. They are Solana-wide constants
// with no per-deployment variation, which is the only reason they are hard-coded
// here while a token's mint address deliberately is not.
var (
	// TokenProgramID is the SPL Token program.
	TokenProgramID = MustPublicKey("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	// Token2022ProgramID is the SPL Token-2022 program.
	Token2022ProgramID = MustPublicKey("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")
	// ATAProgramID derives Associated Token Accounts.
	ATAProgramID = MustPublicKey("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")
	// MetadataProgramID is Metaplex Token Metadata, the only place an SPL token
	// carries a symbol on chain.
	MetadataProgramID = MustPublicKey("metaqbxxUerdq28cj1RbAWkYQm3ybzjb6a8bt518x1s")
)

// pdaMarker is appended to every Program Derived Address preimage so a PDA can
// never collide with a hash used for another purpose.
const pdaMarker = "ProgramDerivedAddress"

// findProgramAddress is Solana's find_program_address: the first candidate,
// counting the bump byte DOWN from 255, that is not a valid ed25519 curve point.
//
// The descent is not an optimisation detail — it is the definition. An address
// that lies on the curve could have a private key, so a program may not own it;
// the canonical PDA is the highest bump that is off the curve, and picking a
// different one derives a different, empty account. Verified against a live
// cluster at bumps 255, 254 and 253 (see pubkey_test.go).
func findProgramAddress(seeds [][]byte, program PublicKey) (PublicKey, error) {
	for bump := 255; bump >= 0; bump-- {
		h := sha256.New()
		for _, s := range seeds {
			h.Write(s)
		}
		h.Write([]byte{byte(bump)})
		h.Write(program[:])
		h.Write([]byte(pdaMarker))
		var out PublicKey
		copy(out[:], h.Sum(nil))
		if !isOnCurve(out) {
			return out, nil
		}
	}
	// 256 consecutive on-curve hashes. Not reachable with SHA-256.
	return PublicKey{}, fmt.Errorf("solanarpc: no off-curve address for the given seeds")
}

// AssociatedTokenAddress derives the ATA that an SPL transfer to `owner` for
// `mint` actually lands in.
//
// tokenProgram is a parameter and not a constant because Token-2022 mints derive
// under their own program id: using the classic program's id for a Token-2022
// mint yields an address that exists nowhere, and the deposits into it would be
// invisible rather than wrong — the worst failure this rail has.
func AssociatedTokenAddress(owner, tokenProgram, mint PublicKey) (PublicKey, error) {
	return findProgramAddress([][]byte{owner[:], tokenProgram[:], mint[:]}, ATAProgramID)
}

// MetadataAddress derives the Metaplex Token Metadata account for a mint.
func MetadataAddress(mint PublicKey) (PublicKey, error) {
	return findProgramAddress([][]byte{[]byte("metadata"), MetadataProgramID[:], mint[:]}, MetadataProgramID)
}

// isOnCurve reports whether the 32 bytes decode to an ed25519 curve point,
// which is the test Solana itself applies when choosing a PDA bump.
//
// The y coordinate is passed through UNREDUCED, and that is a checked
// assumption rather than an oversight. Solana's check runs through
// curve25519-dalek, whose field load masks the sign bit and reads the remainder
// modulo 2^255-19, so a y at or above the prime names the same point as y-p.
// edwards25519 accepts those same non-canonical encodings, so the two libraries
// agree byte for byte and no reduction is needed here.
//
// If that ever stopped being true, this would answer "off the curve" where
// Solana answers "on it" for roughly one candidate in 2^250, pick a different
// bump, and derive an address no deposit ever reaches. Rather than carry
// defensive byte arithmetic no test can distinguish from its absence, the
// assumption is PINNED — see TestIsOnCurveAcceptsNonCanonicalYAsSolanaDoes.
func isOnCurve(k PublicKey) bool {
	_, err := new(edwards25519.Point).SetBytes(k[:])
	return err == nil
}
