package solanarpc

import "testing"

// Real mainnet accounts. Every ATA below was derived by an INDEPENDENT
// implementation and then confirmed against api.mainnet-beta.solana.com with
// getTokenAccountsByOwner — the node agreed on all three. They are here because
// a derivation that is merely self-consistent proves nothing: an ATA nobody
// funds is indistinguishable from a customer who never paid.
const (
	usdcMint = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

	// The three bumps below are not decoration. find_program_address counts DOWN
	// from 255 and stops at the first off-curve candidate, so these exercise
	// zero, one and two on-curve rejections — the loop is really running.
	ownerBump255 = "GJRs4FwHtemZ5ZE9x3FNvJ8TMwitKTh21yxdRPqn7npE"
	ataBump255   = "DeqZejBFrRwWraY4g4besYmibTY1QcV1Fcg6VfoEvn4T"
	ownerBump254 = "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM"
	ataBump254   = "FGETo8T8wMcN2wCjav8VK6eh3dLk63evNDPxzLSJra8B"
	ownerBump253 = "8meoEbDNDAogUcAm88F5coEASwyLbqcAMr47WwhpukUx"
	ataBump253   = "GLX7bTkwHg52vsqhZqg5h9C78Vg6G63tCqtD8JHCNmTo"

	usdcMetadataPDA = "5x38Kp4hvdomTCnCrAny4UtMUt5rQBdB6px2K1Ui45Wq"
)

func mustKey(t *testing.T, s string) PublicKey {
	t.Helper()
	k, err := ParsePublicKey(s)
	if err != nil {
		t.Fatalf("ParsePublicKey(%q): %v", s, err)
	}
	return k
}

func TestParsePublicKey_RoundTrips(t *testing.T) {
	if got := mustKey(t, usdcMint).String(); got != usdcMint {
		t.Fatalf("round trip changed the address: %q → %q", usdcMint, got)
	}
}

// base58 carries no length, so a truncated paste decodes cleanly into the wrong
// number of bytes. Anything that is not exactly 32 must be refused HERE, where
// it is a configuration error, rather than becoming a derivation seed.
func TestParsePublicKey_RefusesAnythingButThirtyTwoBytes(t *testing.T) {
	for _, tc := range []struct{ name, in string }{
		{"empty", ""},
		{"truncated", "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt"},
		{"an EVM address", "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"},
		{"not base58 (contains 0)", "0PjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"},
		{"a 64-byte signature", "5vbwcZLXmcGDyT459oYGdBHx5dGKXY9tL5NxYQbxTwtorUNiGCxmu4hAtu3K8CtmzTyMnuZBLGKTXpfNhmCZgc1c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if k, err := ParsePublicKey(tc.in); err == nil {
				t.Fatalf("accepted %q as the address %s", tc.in, k)
			}
		})
	}
}

// THE test for this file. An SPL transfer never lands at the address the
// customer is shown — it lands in the ATA derived from it — so a watcher that
// gets this wrong sees an empty account forever while the money sits one
// derivation away. Verified against the live cluster.
func TestAssociatedTokenAddress_MatchesTheCluster(t *testing.T) {
	mint := mustKey(t, usdcMint)
	for _, tc := range []struct{ name, owner, want string }{
		{"bump 255", ownerBump255, ataBump255},
		{"bump 254", ownerBump254, ataBump254},
		{"bump 253", ownerBump253, ataBump253},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ata, err := AssociatedTokenAddress(mustKey(t, tc.owner), TokenProgramID, mint)
			if err != nil {
				t.Fatalf("AssociatedTokenAddress: %v", err)
			}
			if got := ata.String(); got != tc.want {
				t.Fatalf("ATA for %s = %s, want %s — deposits to this address would be invisible", tc.owner, got, tc.want)
			}
			if ata.String() == tc.owner {
				t.Fatal("the ATA equals the owner; watching the owner would see nothing")
			}
		})
	}
}

// The token program is a SEED, so a Token-2022 mint derives somewhere else
// entirely. Passing the classic program id for one would produce an address
// that exists nowhere — which is why the client reads the mint's owning program
// before it derives anything.
func TestAssociatedTokenAddress_DependsOnTheTokenProgram(t *testing.T) {
	owner, mint := mustKey(t, ownerBump255), mustKey(t, usdcMint)
	classic, err := AssociatedTokenAddress(owner, TokenProgramID, mint)
	if err != nil {
		t.Fatalf("classic: %v", err)
	}
	t22, err := AssociatedTokenAddress(owner, Token2022ProgramID, mint)
	if err != nil {
		t.Fatalf("token-2022: %v", err)
	}
	if classic == t22 {
		t.Fatal("the token program does not affect the derivation — a Token-2022 mint would be watched at the wrong address")
	}
}

func TestMetadataAddress_MatchesTheCluster(t *testing.T) {
	md, err := MetadataAddress(mustKey(t, usdcMint))
	if err != nil {
		t.Fatalf("MetadataAddress: %v", err)
	}
	if got := md.String(); got != usdcMetadataPDA {
		t.Fatalf("USDC metadata PDA = %s, want %s", got, usdcMetadataPDA)
	}
}

// A PDA is off the curve BY DEFINITION — that is what makes it an address no
// private key can sign for. If the curve test were inverted or stubbed out, the
// derivation would still be deterministic and would still round-trip, and every
// other test here would pass while pointing at the wrong account.
func TestFindProgramAddress_IsOffTheCurve(t *testing.T) {
	for _, owner := range []string{ownerBump255, ownerBump254, ownerBump253} {
		ata, err := AssociatedTokenAddress(mustKey(t, owner), TokenProgramID, mustKey(t, usdcMint))
		if err != nil {
			t.Fatalf("%s: %v", owner, err)
		}
		if isOnCurve(ata) {
			t.Fatalf("derived ATA %s lies on the curve — it is not a program address", ata)
		}
	}
	// And the other direction: a real account key IS a curve point, so the test
	// distinguishes the two cases rather than always answering "off".
	if !isOnCurve(mustKey(t, ownerBump255)) {
		t.Fatal("a wallet address is reported off-curve — the curve test is not testing anything")
	}
}

// The bump search must agree with Solana's for EVERY candidate, including the
// ones no hash will realistically produce.
//
// y = p+k for k in 0..18 are the only 255-bit values at or above the field
// prime. curve25519-dalek — which is what Solana's own is-on-curve test runs
// through — reduces them and answers about the reduced point. A library that
// instead rejected them as non-canonical would answer "off the curve" for all
// nineteen, and this PDA search would silently pick a different bump from
// Solana's for roughly one candidate in 2^250: a deposit address nobody could
// ever fund.
//
// This pins the agreement. If edwards25519 ever starts enforcing canonical
// encodings, this test fails and says exactly what to do about it — which is
// why isOnCurve carries no defensive reduction of its own.
func TestIsOnCurveAcceptsNonCanonicalYAsSolanaDoes(t *testing.T) {
	onCurve := 0
	for k := 0; k <= 18; k++ {
		var y PublicKey
		y[0] = byte(0xed + k) // p = 2^255-19: 0xed, then 0xff…, then 0x7f
		for i := 1; i < 31; i++ {
			y[i] = 0xff
		}
		y[31] = 0x7f
		if isOnCurve(y) {
			onCurve++
		}
	}
	// Twelve of the nineteen reduce to valid curve points; the count is a
	// property of the curve, not of any library's policy.
	if onCurve != 12 {
		t.Fatalf("%d of the 19 encodings with y >= p decode as curve points, want 12 — edwards25519 has begun rejecting non-canonical encodings, so this bump search now disagrees with Solana's and isOnCurve must reduce y before decoding", onCurve)
	}
}
