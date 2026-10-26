// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package circle

import (
	"fmt"
	"strings"
)

// The deployment arms the rail with two plain values and no secret: Gateway
// takes no key for anything the rail calls.
//
//	CIRCLE_GATEWAY_TREASURY  the address payments are made TO — Hanzo's wallet.
//	                         Unset: the rail is shut and nothing below is read.
//	CIRCLE_ENVIRONMENT       "production" (mainnet) or "sandbox" (testnet).
//	                         Required once the treasury is set: which money is
//	                         real is never a default.
const (
	EnvTreasury = "CIRCLE_GATEWAY_TREASURY"
	EnvNetwork  = "CIRCLE_ENVIRONMENT"
)

// Config is the rail as a deployment arms it.
type Config struct {
	Network  Network
	Treasury string // 0x + 40 lowercase hex; empty when the rail is not armed
}

// Armed reports whether a treasury is configured. An unarmed rail offers
// nothing and accepts nothing.
func (c Config) Armed() bool { return c.Treasury != "" }

// FromEnv reads the rail's configuration from environ (as os.Environ returns
// it). No treasury is the inert state, not an error. A treasury with an
// unreadable address or no stated environment IS an error: an operator who
// armed the rail must not get a process that takes payments into an address
// it misread, or on a network it guessed.
func FromEnv(environ []string) (Config, error) {
	env := map[string]string{}
	for _, kv := range environ {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env[k] = strings.TrimSpace(v)
		}
	}
	raw := env[EnvTreasury]
	if raw == "" {
		return Config{}, nil
	}
	addr, err := Address(raw)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", EnvTreasury, err)
	}
	if addr == ([20]byte{}) {
		return Config{}, fmt.Errorf("%s is the zero address", EnvTreasury)
	}
	var n Network
	switch env[EnvNetwork] {
	case "production":
		n = Mainnet
	case "sandbox":
		n = Testnet
	default:
		return Config{}, fmt.Errorf("%s is set, so %s must be \"production\" or \"sandbox\", not %q", EnvTreasury, EnvNetwork, env[EnvNetwork])
	}
	return Config{Network: n, Treasury: Hex(addr[:])}, nil
}
