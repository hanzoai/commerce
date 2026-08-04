// Copyright © 2026 Hanzo AI. MIT License.

package middleware

import (
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/zap-proto/zip"
)

// MintRoute is one money-mint route as registered: the HTTP method and the full
// path fiber routes on (e.g. {POST, "/v1/billing/deposit"}). It is the exported
// shape of the mint surface — see MintRoutes.
type MintRoute struct {
	Method string
	Path   string
}

var (
	mintMu       sync.Mutex
	mintRegistry = map[MintRoute]struct{}{}
)

// Mint returns a view of r on which EVERY registered route is a money-mint
// route: it prepends PlatformOnly() to the route's handler chain AND records the
// route in the mint registry (MintRoutes). Registration is the single
// declaration — the gate and the registry entry are the same act, so they cannot
// drift:
//
//	mint := middleware.Mint(api)
//	mint.Post("/deposit", Deposit)   // gated AND recorded
//
// This replaces the CONVENTION `api.Post("/deposit", mintRequired, Deposit)`,
// where "this is a mint route" was knowable only by reading the middleware chain
// and a downstream consumer (cloud's billing-bridge allowlist) had to hand-copy
// the list. The gate's BEHAVIOR is unchanged: PlatformOnly is prepended, so it
// runs FIRST and the handler LAST, exactly as the explicit chain did.
//
// The natural shape for an authz class is a group carrying its own Use
// (`api.Use(adminRequired)` one level up). Mint deliberately does NOT do that,
// because this router cannot express it: fiber's Use matches by PATH PREFIX, not
// by group membership, and a bare sub-group inherits its parent's prefix
// verbatim — so `api.Group("").Use(PlatformOnly())` gates every neighbouring
// route under /v1/billing, including the org-admin reads that must stay
// reachable. TestGroupUseIsPrefixScopedNotMembership proves it and fails if that
// scoping ever changes (at which point the gated-sub-group shape becomes right).
// Mint therefore prepends the gate to each route's own chain, which gates exactly
// the declared routes and nothing else.
//
// Register it on a router that has ALREADY resolved the caller — PlatformOnly
// reads what TokenRequired sets and only ever NARROWS (see platformonly.go).
func Mint(r zip.Router) zip.Router {
	return &mintRouter{inner: r, prefix: groupPrefix(r), gate: PlatformOnly()}
}

// MintRoutes returns every route declared through Mint, sorted and deduplicated
// — the mint surface, DERIVED from the registrations themselves rather than
// hand-listed. It is exported for cross-service checks: cloud's /v1/billing
// bridge forwards with the admin COMMERCE_SERVICE_TOKEN, which satisfies
// MayMintMoney, so its forwardable allowlist MUST stay disjoint from this set.
//
// It reports what has been REGISTERED in this process: entries appear as Route()
// runs, so call it once the routes are registered. Registering the same route
// again is idempotent (the registry is a set), and the paths are full paths, so
// a caller comparing against a subtree should match on that subtree's prefix.
func MintRoutes() []MintRoute {
	mintMu.Lock()
	defer mintMu.Unlock()

	out := make([]MintRoute, 0, len(mintRegistry))
	for r := range mintRegistry {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}

// mintRouter decorates a zip.Router so each registration is gated and recorded.
type mintRouter struct {
	inner  zip.Router
	prefix string
	gate   zip.Handler
}

// add records method+full path, then registers the route on the inner router
// with the gate FIRST and the caller's chain after it (zip runs the chain in
// argument order), which is byte-for-byte the shape the explicit `mintRequired,
// Handler` chain produced.
func (m *mintRouter) add(method string, register func(string, ...zip.Handler) zip.Router, path string, handlers []zip.Handler) zip.Router {
	mintMu.Lock()
	mintRegistry[MintRoute{Method: method, Path: joinPath(m.prefix, path)}] = struct{}{}
	mintMu.Unlock()

	register(path, append([]zip.Handler{m.gate}, handlers...)...)
	return m
}

func (m *mintRouter) Get(p string, h ...zip.Handler) zip.Router {
	return m.add(http.MethodGet, m.inner.Get, p, h)
}
func (m *mintRouter) Post(p string, h ...zip.Handler) zip.Router {
	return m.add(http.MethodPost, m.inner.Post, p, h)
}
func (m *mintRouter) Put(p string, h ...zip.Handler) zip.Router {
	return m.add(http.MethodPut, m.inner.Put, p, h)
}
func (m *mintRouter) Patch(p string, h ...zip.Handler) zip.Router {
	return m.add(http.MethodPatch, m.inner.Patch, p, h)
}
func (m *mintRouter) Delete(p string, h ...zip.Handler) zip.Router {
	return m.add(http.MethodDelete, m.inner.Delete, p, h)
}
func (m *mintRouter) Head(p string, h ...zip.Handler) zip.Router {
	return m.add(http.MethodHead, m.inner.Head, p, h)
}
func (m *mintRouter) Options(p string, h ...zip.Handler) zip.Router {
	return m.add(http.MethodOptions, m.inner.Options, p, h)
}
func (m *mintRouter) All(p string, h ...zip.Handler) zip.Router {
	return m.add("ALL", m.inner.All, p, h)
}

// Group returns a mint view of the sub-group: every route under it is gated and
// recorded, so a whole mint subtree declares itself the same one way.
func (m *mintRouter) Group(prefix string, handlers ...zip.Handler) zip.Router {
	inner := m.inner.Group(prefix, handlers...)
	return &mintRouter{inner: inner, prefix: groupPrefix(inner), gate: m.gate}
}

// Use is refused: Mint gates per-route precisely because fiber's Use is
// prefix-scoped and would leak the gate onto neighbouring non-mint routes. A
// middleware meant for the whole group belongs on the group itself, before Mint
// wraps it. Boot-time panic, never a request-time surprise — the same contract
// zip applies to a route registered with no handler.
func (m *mintRouter) Use(handlers ...zip.Component) zip.Router {
	panic("middleware.Mint: Use is not supported — Mint gates per-route; apply shared middleware to the underlying group before wrapping it with Mint")
}

// OpScope carries the gate onto a TYPED op declared on this router, so
// `zip.Post(mint, "/deposit", Deposit)` is gated exactly as `mint.Post` is.
//
// It is not optional and it is not a formality. zip asks a Router where an op it
// declares should land, and a decorator that answered with the inner router's
// scope would register a money-mint op with NO gate — this router's entire
// purpose, skipped silently, on the routes that move money. Folding m.gate into
// the scope's middleware is what keeps "registration IS the gate" true for the
// typed registrars as well as the chainable ones.
//
// Use [MintOp] to declare one: it records the route in the registry too, which
// this method cannot do because zip asks for the scope without saying which path
// is about to be registered under it. A bare zip.Post through a mint router is
// gated but does not appear in [MintRoutes].
func (m *mintRouter) OpScope() zip.OpScope {
	s := m.inner.OpScope()
	if s.Middleware == nil {
		s.Middleware = zip.Chain(m.gateMW())
		return s
	}
	s.Middleware = zip.Chain(s.Middleware, m.gateMW())
	return s
}

// gateMW is the mint gate in middleware shape. It delegates to PlatformOnlyMW
// rather than calling m.gate: m.gate is PlatformOnly, whose continuation is
// c.Next() — fiber's chain — and on the typed-op path the handler is wrapped
// INSIDE this middleware, not chained after it. c.Next() therefore finds nothing
// to run, yields 404, and the gate reports that as an error, so an authorized
// request to a typed mint op never reaches its handler.
func (m *mintRouter) gateMW() zip.Middleware {
	return PlatformOnlyMW
}

// MintOp declares a TYPED op on a mint router: gated and recorded, the same
// single declaration `mint.Post(path, h)` makes for an untyped route, and
// projected into the OpenAPI document, the MCP tool list, the CLI and the
// op-call plane like any other typed op.
//
// It is a function rather than a method because Go methods cannot take type
// parameters — the same reason zip.Post is one. Pass the mint router as `on`:
//
//	middleware.MintOp(mint, http.MethodPost, "/deposit", Deposit)
//
// A money route that is not typed is invisible to every projection, which is why
// this exists; a money route that is typed but declared with a bare zip.Post is
// gated (see [mintRouter.OpScope]) but missing from [MintRoutes], which is why
// this is the one to use.
func MintOp[In, Out any](on zip.Router, method, path string, fn zip.TypedHandler[In, Out], opts ...zip.OpOption) {
	if m, ok := on.(*mintRouter); ok {
		mintMu.Lock()
		mintRegistry[MintRoute{Method: method, Path: joinPath(m.prefix, path)}] = struct{}{}
		mintMu.Unlock()
	}
	switch method {
	case http.MethodGet:
		zip.Get(on, path, fn, opts...)
	case http.MethodPost:
		zip.Post(on, path, fn, opts...)
	case http.MethodPut:
		zip.Put(on, path, fn, opts...)
	case http.MethodPatch:
		zip.Patch(on, path, fn, opts...)
	case http.MethodDelete:
		zip.Delete(on, path, fn, opts...)
	default:
		panic("middleware.MintOp: unsupported method " + method)
	}
}

// groupPrefix reports the full prefix of r, read from the SAME value
// fiber routes on (Group.Prefix, which fiber builds by joining parents). A root
// app router has no prefix.
func groupPrefix(r zip.Router) string { return r.OpScope().Prefix }

// joinPath mirrors fiber's getGroupPath, so a recorded path equals the path
// fiber actually routes.
func joinPath(prefix, path string) string {
	if path == "" {
		return prefix
	}
	if path[0] != '/' {
		path = "/" + path
	}
	return strings.TrimRight(prefix, "/") + path
}
