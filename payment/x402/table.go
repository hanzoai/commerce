// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package x402

import (
	"fmt"
	"maps"
	"math/big"
	"slices"
	"strings"
	"sync"
)

// Asset is one way to be paid: a token on a network, the payee, and how long a
// payer has to sign. Every field is configuration; none has a default, because
// a default token or payee would be a value this package invented.
type Asset struct {
	// Network is the CAIP-2 identifier, e.g. "eip155:8453".
	Network string
	// Address is the token contract. It must implement EIP-3009.
	Address string
	// Name and Version are the token's EIP-712 domain name() and version(): the
	// payer signs over them, so they must be what the token itself answers.
	Name    string
	Version string
	// Decimals is the token's decimals(), the scale of Price amounts.
	Decimals int
	// PayTo is the payee address.
	PayTo string
	// MaxTimeoutSeconds is how long the payer has to complete the payment.
	MaxTimeoutSeconds int64
}

// Validate checks an Asset at the boundary, so a bad configuration fails at
// start and never in front of a payer.
func (a Asset) Validate() error {
	if _, err := ChainID(a.Network); err != nil {
		return err
	}
	if _, err := Address(a.Address); err != nil {
		return fmt.Errorf("x402: asset address: %w", err)
	}
	if _, err := Address(a.PayTo); err != nil {
		return fmt.Errorf("x402: asset payTo: %w", err)
	}
	if a.Name == "" || a.Version == "" {
		return fmt.Errorf("x402: asset %s needs the token's EIP-712 name and version", a.Address)
	}
	if a.Decimals < 0 || a.Decimals > 77 {
		return fmt.Errorf("x402: asset %s decimals %d out of range", a.Address, a.Decimals)
	}
	if a.MaxTimeoutSeconds <= 0 {
		return fmt.Errorf("x402: asset %s needs maxTimeoutSeconds", a.Address)
	}
	return nil
}

// Requirements states what paying amount (atomic units) to payTo costs on this
// asset. An empty payTo means the asset's own.
func (a Asset) Requirements(amount, payTo string) PaymentRequirements {
	if payTo == "" {
		payTo = a.PayTo
	}
	return PaymentRequirements{
		Scheme:            SchemeExact,
		Network:           a.Network,
		Amount:            amount,
		Asset:             a.Address,
		PayTo:             payTo,
		MaxTimeoutSeconds: a.MaxTimeoutSeconds,
		Extra:             &Extra{AssetTransferMethod: TransferEIP3009, Name: a.Name, Version: a.Version},
	}
}

// Atomic converts a decimal amount in whole units ("0.01") to atomic units at
// decimals ("10000" at 6). It refuses what it would have to round: an amount
// with more fractional digits than the token has, a negative, a zero, or
// anything that is not plain decimal digits.
func Atomic(amount string, decimals int) (string, error) {
	whole, frac, _ := strings.Cut(strings.TrimSpace(amount), ".")
	if whole == "" && frac == "" || !digitsOnly(whole) || !digitsOnly(frac) {
		return "", fmt.Errorf("x402: %q is not a decimal amount", amount)
	}
	if len(frac) > decimals {
		return "", fmt.Errorf("x402: %q has more than %d fractional digits", amount, decimals)
	}
	v, ok := new(big.Int).SetString(whole+frac+strings.Repeat("0", decimals-len(frac)), 10)
	if !ok || v.Sign() <= 0 {
		return "", fmt.Errorf("x402: %q is not a positive amount", amount)
	}
	if v.Cmp(maxWord) > 0 {
		return "", fmt.Errorf("x402: %q overflows uint256", amount)
	}
	return v.String(), nil
}

func digitsOnly(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// Price is what one resource costs, in whole units of every asset the Table
// accepts: "0.01" is a cent when the assets are dollar stablecoins.
type Price struct {
	Amount      string
	Description string
	MimeType    string
	// PayTo overrides the asset's payee for this resource; empty keeps it.
	PayTo string
}

// Table is the resource price table: which resources are for sale, at what
// price, in which assets. Resources are named by the caller, a route pattern or
// any other key. It is safe for concurrent use.
type Table struct {
	assets []Asset
	mu     sync.RWMutex
	rows   map[string]Price
}

// NewTable returns an empty table that quotes every price in each of assets, in
// order. At least one asset is required and each must validate.
func NewTable(assets ...Asset) (*Table, error) {
	if len(assets) == 0 {
		return nil, fmt.Errorf("x402: a table needs at least one asset")
	}
	for _, a := range assets {
		if err := a.Validate(); err != nil {
			return nil, err
		}
	}
	return &Table{assets: append([]Asset(nil), assets...), rows: map[string]Price{}}, nil
}

// Set prices a resource, replacing any earlier price. The amount must convert to
// atomic units on every asset without rounding.
func (t *Table) Set(resource string, p Price) error {
	if strings.TrimSpace(resource) == "" {
		return fmt.Errorf("x402: a priced resource needs a name")
	}
	if p.PayTo != "" {
		if _, err := Address(p.PayTo); err != nil {
			return fmt.Errorf("x402: %s payTo: %w", resource, err)
		}
	}
	for _, a := range t.assets {
		if _, err := Atomic(p.Amount, a.Decimals); err != nil {
			return fmt.Errorf("x402: %s on %s: %w", resource, a.Network, err)
		}
	}
	t.mu.Lock()
	t.rows[resource] = p
	t.mu.Unlock()
	return nil
}

// Remove takes a resource off sale.
func (t *Table) Remove(resource string) {
	t.mu.Lock()
	delete(t.rows, resource)
	t.mu.Unlock()
}

// Priced reports whether the resource is for sale.
func (t *Table) Priced(resource string) bool {
	t.mu.RLock()
	_, ok := t.rows[resource]
	t.mu.RUnlock()
	return ok
}

// Quote is a resource's terms: one requirement per asset, and its description.
func (t *Table) Quote(resource string) (accepts []PaymentRequirements, info ResourceInfo, ok bool) {
	t.mu.RLock()
	p, ok := t.rows[resource]
	t.mu.RUnlock()
	if !ok {
		return nil, ResourceInfo{}, false
	}
	accepts, info = t.quote(p)
	return accepts, info, true
}

func (t *Table) quote(p Price) ([]PaymentRequirements, ResourceInfo) {
	accepts := make([]PaymentRequirements, 0, len(t.assets))
	for _, a := range t.assets {
		atomic, _ := Atomic(p.Amount, a.Decimals) // Set validated it
		accepts = append(accepts, a.Requirements(atomic, p.PayTo))
	}
	return accepts, ResourceInfo{Description: p.Description, MimeType: p.MimeType}
}

// Row is one priced resource as a listing publishes it.
type Row struct {
	Resource string                `json:"resource"`
	Price    string                `json:"price"`
	Accepts  []PaymentRequirements `json:"accepts"`
}

// Listing is every resource for sale, ordered by name, never nil.
func (t *Table) Listing() []Row {
	t.mu.RLock()
	rows := maps.Clone(t.rows)
	t.mu.RUnlock()
	out := make([]Row, 0, len(rows))
	for _, name := range slices.Sorted(maps.Keys(rows)) {
		accepts, _ := t.quote(rows[name])
		out = append(out, Row{Resource: name, Price: rows[name].Amount, Accepts: accepts})
	}
	return out
}
