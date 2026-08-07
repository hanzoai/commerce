// Package tonrpc reads TON jetton deposits for ONE jetton master over the TON
// Index (toncenter v3) HTTP API. It is the TON half of the crypto top-up rail's
// read side, beside billing/husdindex (EVM), billing/solanarpc (Solana) and
// billing/xrplrpc (XRPL), and it satisfies depositwatch.Reader like all three.
//
// WHY AN INDEXER AND NOT A NODE. TON's node RPC has no per-account transaction
// index worth the name, and — more to the point — TON is SHARDED: the basechain
// splits and merges shards under load, so "the block number" is not a single
// global sequence the way an EVM assumes. What IS a single global sequence is
// the MASTERCHAIN seqno, and a transaction becomes final exactly when the
// masterchain block that references its shard block is committed. The index
// records that block per transaction (mc_block_seqno), which is what lets this
// client answer the watcher in the shape it asks for. See Client.BlockNumber.
package tonrpc

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// Address is a TON account: a workchain and a 32-byte account hash.
//
// It is parsed to those two values rather than kept as a string because a TON
// account has SEVERAL correct spellings — bounceable (EQ…), non-bounceable
// (UQ…), testnet-flagged, and the raw <workchain>:<hex> form the index answers
// in — and they are all the same account. Comparing the spellings would make
// one custody address look like four, so every comparison in this package is on
// the parsed value and the canonical rendering is derived from it.
type Address struct {
	Workchain int32
	Hash      [32]byte
}

// String renders the canonical RAW form, which is what the index uses:
// "<workchain>:<64 uppercase hex>".
func (a Address) String() string {
	return strconv.FormatInt(int64(a.Workchain), 10) + ":" + strings.ToUpper(hex.EncodeToString(a.Hash[:]))
}

const (
	// userFriendlyLen is the decoded length of the EQ…/UQ… form: tag, workchain,
	// 32-byte hash, CRC-16.
	userFriendlyLen = 36
	// tagBounceable and tagNonBounceable are the two address tags; the 0x80 bit
	// on top of either marks an address as testnet-only.
	tagBounceable    = 0x11
	tagNonBounceable = 0x51
	tagTestOnly      = 0x80
)

// ParseAddress reads a TON address in either form it is published in.
//
// The user-friendly form carries a CRC-16 over its own bytes, and it is
// CHECKED. That check is the whole reason this function exists rather than a
// string comparison: it turns a mistyped custody address into a boot failure
// instead of a deposit nobody ever sees. The raw form has no checksum to
// verify — it is what the index emits, not what a human types — so it is
// validated on shape alone.
//
// A TESTNET-flagged address is refused outright. It is a different network's
// account that happens to be spelled almost identically, and crediting mainnet
// balance against a testnet address is a mistake worth failing loudly on.
func ParseAddress(s string) (Address, error) {
	var a Address
	s = strings.TrimSpace(s)
	if s == "" {
		return a, fmt.Errorf("tonrpc: empty address")
	}
	if wc, hash, ok := strings.Cut(s, ":"); ok {
		n, err := strconv.ParseInt(wc, 10, 32)
		if err != nil {
			return a, fmt.Errorf("tonrpc: %q has no readable workchain: %w", s, err)
		}
		raw, err := hex.DecodeString(hash)
		if err != nil || len(raw) != 32 {
			return a, fmt.Errorf("tonrpc: %q is not <workchain>:<64 hex>", s)
		}
		a.Workchain = int32(n)
		copy(a.Hash[:], raw)
		return a, nil
	}

	raw, err := decodeBase64Either(s)
	if err != nil {
		return a, fmt.Errorf("tonrpc: %q is not a base64 TON address: %w", s, err)
	}
	if len(raw) != userFriendlyLen {
		return a, fmt.Errorf("tonrpc: %q decodes to %d bytes, not the %d of a user-friendly TON address", s, len(raw), userFriendlyLen)
	}
	tag := raw[0]
	if tag&tagTestOnly != 0 {
		return a, fmt.Errorf("tonrpc: %q is a TESTNET-only address", s)
	}
	if tag != tagBounceable && tag != tagNonBounceable {
		return a, fmt.Errorf("tonrpc: %q has address tag 0x%02x, which is neither bounceable (0x%02x) nor non-bounceable (0x%02x)", s, tag, tagBounceable, tagNonBounceable)
	}
	if got, want := crc16XModem(raw[:34]), uint16(raw[34])<<8|uint16(raw[35]); got != want {
		return a, fmt.Errorf("tonrpc: %q fails its CRC — it is a mistyped address, not an account", s)
	}
	// The workchain byte is SIGNED: 0x00 is the basechain and 0xFF is the
	// masterchain (−1). Reading it unsigned would turn the masterchain into
	// workchain 255, an account that does not exist.
	a.Workchain = int32(int8(raw[1]))
	copy(a.Hash[:], raw[2:34])
	return a, nil
}

// decodeBase64Either decodes url-safe base64 (what TON publishes) or standard
// base64 (what some tools emit), because both spellings of the same address are
// pasted into configuration by real people.
func decodeBase64Either(s string) ([]byte, error) {
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(s)
}

// crc16XModem is CRC-16/XMODEM (polynomial 0x1021, zero init, no reflection,
// no final xor) — the checksum TON's user-friendly address format uses.
func crc16XModem(b []byte) uint16 {
	var crc uint16
	for _, v := range b {
		crc ^= uint16(v) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// decodeHash reads a 32-byte hash the index rendered as base64, and returns it
// as LOWERCASE HEX.
//
// The rendering is normalised here, and nowhere else, because a transaction id
// goes into the dedup key — the thing that makes a credit happen exactly once
// — and that key must be a function of the EVENT and not of how an endpoint
// chose to print it. This API prints hashes in base64; others print hex; a
// re-scan through either must land on the same ledger row.
func decodeHash(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty hash")
	}
	if len(s) == 64 {
		if b, err := hex.DecodeString(s); err == nil {
			return hex.EncodeToString(b), nil
		}
	}
	b, err := decodeBase64Either(s)
	if err != nil {
		return "", fmt.Errorf("%q is neither hex nor base64: %w", s, err)
	}
	if len(b) != 32 {
		return "", fmt.Errorf("%q decodes to %d bytes, not a 256-bit hash", s, len(b))
	}
	return hex.EncodeToString(b), nil
}
