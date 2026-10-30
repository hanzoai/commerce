// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"crypto/ecdsa"
	"fmt"
	"strings"

	"github.com/luxfi/crypto"

	"github.com/hanzoai/commerce/payment/eip3009"
)

// The deployment arms the rail with three values:
//
//	USDC_TREASURY     the address payments are made TO — Hanzo's wallet.
//	                  Unset: the rail is shut and nothing below is read.
//	USDC_NETWORK      "mainnet" or "testnet". Required once the treasury is
//	                  set: which money is real is never a default.
//	USDC_RELAYER_KEY  the relayer's secp256k1 key, hex. It pays the gas of
//	                  every transferWithAuthorization the rail submits and
//	                  holds nothing else: the payer's USDC moves straight from
//	                  the payer to the treasury. A secret, so it comes from KMS
//	                  (/commerce-secrets/USDC_RELAYER_KEY) and nowhere else.
//
// Each network is offered only where two independent endpoints are configured:
// its RPC, under the name the deposit watcher reads it by
// (CRYPTO_DEPOSIT_RPC_BASE, …; one endpoint per chain for both crypto rails),
// and a WITNESS (USDC_WITNESS_RPC_BASE, …) from another provider, which must
// agree with it before anything is credited. One endpoint is one party's word
// about whether money arrived.
const (
	EnvTreasury = "USDC_TREASURY"
	EnvNetwork  = "USDC_NETWORK"
	EnvRelayer  = "USDC_RELAYER_KEY"
)

// Chain is one network the rail relays USDC on: the token, its EIP-712 domain
// as the token states it (name(), version()), and how deep a transfer must be
// before it is credited. Written here and reviewed, never read from a peer: the
// contract and the domain are what a payer's signature commits to, and the
// rail checks both against the chain itself before it offers the network
// (ready).
type Chain struct {
	ID            string // CAIP-2, "eip155:8453"
	Name          string // the deposit watcher's chain name, which names its RPC
	ChainID       int64
	USDC          string // lowercase 0x
	Token         string // the token's name() in its EIP-712 domain
	Version       string // the token's version() in its EIP-712 domain
	Confirmations int64
	RPC           string
	Witness       string
}

// Domain is the EIP-712 domain a payment on c is signed under: the token's own.
func (c Chain) Domain() eip3009.Domain {
	return eip3009.Domain{Name: c.Token, Version: c.Version, ChainID: c.ChainID, VerifyingContract: c.USDC}
}

// Circle-issued native USDC only, never a same-symbol substitute: each address
// and domain is Circle's (https://developers.circle.com/stablecoins/usdc-contract-addresses)
// and was read off the chain on 2026-10-30 — name(), version(), decimals() 6,
// and a DOMAIN_SEPARATOR() equal to the one computed from them. Base only: on
// Base the relayer's gas for one transferWithAuthorization is a fraction of a
// cent, where on Ethereum it can exceed a $1 payment. A network joins by a
// reviewed row here.
var (
	mainnet = []Chain{{
		ID: "eip155:8453", Name: "base", ChainID: 8453,
		USDC: "0x833589fcd6edb6e08f4c7c32d4f71b54bda02913", Token: "USD Coin", Version: "2",
		Confirmations: 20, // the deposit watcher's depth on Base
	}}
	testnet = []Chain{{
		ID: "eip155:84532", Name: "base-sepolia", ChainID: 84532,
		USDC: "0x036cbd53842c5426634e7929541ec2318f3dcf7e", Token: "USDC", Version: "2",
		Confirmations: 5,
	}}
)

// Config is the rail as a deployment arms it.
type Config struct {
	Testnet  bool
	Treasury string // 0x + 40 lowercase hex; empty when the rail is not armed
	Relayer  *ecdsa.PrivateKey
	Chains   []Chain // the networks whose RPC is configured
}

// Armed reports whether a treasury is configured. An unarmed rail offers
// nothing and accepts nothing.
func (c Config) Armed() bool { return c.Treasury != "" && c.Relayer != nil && len(c.Chains) > 0 }

// RelayerAddress is the address the relayer key controls, lowercase 0x.
func (c Config) RelayerAddress() string {
	if c.Relayer == nil {
		return ""
	}
	return eip3009.Hex(crypto.PubkeyToAddress(c.Relayer.PublicKey).Bytes())
}

// FromEnv reads the rail's configuration from environ (as os.Environ returns
// it). No treasury is the inert state, not an error. A treasury with an
// unreadable address, no stated network, no usable relayer key or no RPC for
// any network IS an error: an operator who armed the rail must not get a
// process that takes payments into an address it misread, on a network it
// guessed, or that it cannot relay.
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
	addr, err := eip3009.Address(raw)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", EnvTreasury, err)
	}
	if addr == ([20]byte{}) {
		return Config{}, fmt.Errorf("%s is the zero address", EnvTreasury)
	}
	var table []Chain
	cfg := Config{Treasury: eip3009.Hex(addr[:])}
	switch env[EnvNetwork] {
	case "mainnet":
		table = mainnet
	case "testnet":
		table, cfg.Testnet = testnet, true
	default:
		return Config{}, fmt.Errorf("%s is set, so %s must be \"mainnet\" or \"testnet\", not %q", EnvTreasury, EnvNetwork, env[EnvNetwork])
	}
	key, err := crypto.HexToECDSA(strings.TrimPrefix(env[EnvRelayer], "0x"))
	if err != nil {
		// The value is a secret: the error names the variable, never its content.
		return Config{}, fmt.Errorf("%s is set, so %s must be a secp256k1 key in hex", EnvTreasury, EnvRelayer)
	}
	cfg.Relayer = key
	if cfg.RelayerAddress() == cfg.Treasury {
		return Config{}, fmt.Errorf("%s must not be the treasury's key: the relayer is a hot key that pays gas, the treasury holds the money", EnvRelayer)
	}
	for _, c := range table {
		name := strings.ToUpper(strings.ReplaceAll(c.Name, "-", "_"))
		c.RPC, c.Witness = env["CRYPTO_DEPOSIT_RPC_"+name], env["USDC_WITNESS_RPC_"+name]
		switch {
		case c.RPC == "" && c.Witness == "":
			continue
		case c.RPC == "" || c.Witness == "":
			return Config{}, fmt.Errorf("%s needs both CRYPTO_DEPOSIT_RPC_%s and USDC_WITNESS_RPC_%s: a credit is checked against two providers", c.ID, name, name)
		case c.RPC == c.Witness:
			return Config{}, fmt.Errorf("USDC_WITNESS_RPC_%s is the same endpoint as CRYPTO_DEPOSIT_RPC_%s; a witness is a second provider", name, name)
		}
		cfg.Chains = append(cfg.Chains, c)
	}
	if len(cfg.Chains) == 0 {
		return Config{}, fmt.Errorf("%s is set, so at least one of its networks needs an RPC and a witness (CRYPTO_DEPOSIT_RPC_<CHAIN>, USDC_WITNESS_RPC_<CHAIN>)", EnvTreasury)
	}
	return cfg, nil
}
