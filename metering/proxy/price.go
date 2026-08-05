// Package proxy is the ONE way a non-Go Hanzo product (vector, search, or any
// HTTP upstream) opts into prepaid pay-for-everything: a reverse proxy that
// fronts the upstream, gates every request on the caller's commerce balance
// (fail-closed) and records per-org usage after a successful response. It reuses
// metering.Middleware verbatim — the proxy adds only (a) the upstream forwarder
// and (b) an env-driven price table, so a single binary commercializes every
// product without per-product Go code.
//
// It is stdlib-only (net/http, net/http/httputil), matching the metering
// package's leaf posture: deploying it adds no dependency graph to a product —
// it runs as a sidecar in the product's pod.
package proxy

import (
	"net/http"
	"strconv"
	"strings"
)

// Rule prices one class of request. A request matches when its method is in
// Methods (empty = any method) and its path has prefix PathPrefix (empty =
// any path). The first matching rule in a PriceTable wins, so order rules
// most-specific-first. Cents is the flat per-request charge applied on a
// successful (2xx/3xx) response; non-success responses are never charged.
//
// This expresses the billable unit declaratively: "POST /collections/*/points
// costs 2c" (a vector upsert) vs "POST /indexes/*/search costs 1c" (a search
// query). The product's billable unit is config, not code.
type Rule struct {
	Methods    []string
	PathPrefix string
	Cents      int64
}

func (r Rule) matches(method, path string) bool {
	if len(r.Methods) > 0 {
		ok := false
		for _, m := range r.Methods {
			if strings.EqualFold(m, method) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if r.PathPrefix != "" && !strings.HasPrefix(path, r.PathPrefix) {
		return false
	}
	return true
}

// PriceTable is an ordered list of rules plus a default charge for requests
// that match no rule. It is the product-specific billing policy, supplied to
// the proxy. A zero Default with no matching rule records nothing (free path).
type PriceTable struct {
	Rules   []Rule
	Default int64
}

// Price returns the cents to charge for a completed request. Only successful
// outcomes are billed (2xx/3xx); errors return 0 so a failed call is free,
// matching the LLM gate (which records only status=="success").
func (t PriceTable) Price(method, path string, status int) int64 {
	if status < 200 || status >= 400 {
		return 0
	}
	for _, r := range t.Rules {
		if r.matches(method, path) {
			return r.Cents
		}
	}
	return t.Default
}

// ParsePriceTable builds a PriceTable from a compact spec string, so the price
// policy is operator config (an env var the operator wires per product), never
// a code change. Grammar — rules separated by ';', fields within a rule by '|':
//
//	METHODS|PATHPREFIX|CENTS ; METHODS|PATHPREFIX|CENTS ; ... ; default:CENTS
//
// METHODS is a comma list (or '*'/empty for any). PATHPREFIX is a literal path
// prefix (or '*'/empty for any). CENTS is a non-negative integer. A bare
// "default:N" sets the no-match charge. Whitespace around tokens is trimmed.
// Example (vector): "POST|/collections|2 ; *|/collections|1 ; default:0"
//   - charge 2c for any POST under /collections (writes/upserts),
//   - 1c for any other /collections call (reads/search),
//   - nothing otherwise.
//
// Unparseable or empty input yields an empty table (Default 0); callers treat
// that as "nothing to charge", which is safe (never over-bills) — the gate
// still runs, so balance is still enforced even with a misconfigured table.
func ParsePriceTable(spec string) PriceTable {
	var t PriceTable
	for _, part := range strings.Split(spec, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if rest, ok := cutPrefixFold(part, "default:"); ok {
			if c, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64); err == nil && c >= 0 {
				t.Default = c
			}
			continue
		}
		fields := strings.Split(part, "|")
		if len(fields) != 3 {
			continue
		}
		cents, err := strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64)
		if err != nil || cents < 0 {
			continue
		}
		t.Rules = append(t.Rules, Rule{
			Methods:    parseMethods(fields[0]),
			PathPrefix: parsePrefix(fields[1]),
			Cents:      cents,
		})
	}
	return t
}

func parseMethods(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" {
		return nil
	}
	var out []string
	for _, m := range strings.Split(s, ",") {
		if m = strings.TrimSpace(m); m != "" {
			out = append(out, m)
		}
	}
	return out
}

func parsePrefix(s string) string {
	if s = strings.TrimSpace(s); s == "*" {
		return ""
	}
	return s
}

func cutPrefixFold(s, prefix string) (string, bool) {
	if len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
		return s[len(prefix):], true
	}
	return "", false
}

// statusCode reads a captured status from a Skip/Price hook context. It is a
// tiny helper so PriceFunc adapters stay one-liners.
func clampStatus(status int) int {
	if status == 0 {
		return http.StatusOK
	}
	return status
}
