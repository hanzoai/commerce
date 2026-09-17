// Copyright © 2026 Hanzo AI. MIT License.

package middleware

import (
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/zap-proto/zip"
)

// MintRoute is one money route as served: the method and the full path the
// router matches (e.g. {POST, "/v1/billing/deposit"}). It is the exported shape
// of the mint surface — see MintRoutes.
type MintRoute struct {
	Method string
	Path   string
}

var (
	mintMu       sync.Mutex
	mintRegistry = map[MintRoute]struct{}{}
)

// Money is the mint surface: a group whose every leaf carries the platform gate,
// together with the address its routes are served at.
//
// Declaring through it is ONE act with two effects — the gate and the registry
// entry — so they cannot drift apart. That matters because the registry is what
// cloud's billing bridge is checked disjoint from, and a money route that gates
// but does not record reads to that check as though it were not money at all.
//
// The verbs below shadow the group's own so the recording cannot be bypassed by
// reaching for the promoted method. Everything else about the group — Use, With,
// nested Group, the typed projections — is promoted and unchanged.
type Money struct {
	*zip.Group
	mount string
}

// Mint returns the Money surface over r, gated.
//
// The gate wraps the TERMINAL handler, so a typed operation is gated exactly as a
// raw route is, and a neighbour registered against the parent is untouched. That
// last part is the difficulty: fiber's Use matches a path PREFIX and a bare
// sub-group inherits its parent's prefix verbatim, so the intuitive
// `api.Group("").Use(gate)` runs for every route under the parent — which would
// spread the gate across the whole billing surface and refuse the org-admin reads
// that must stay reachable. zip composes a chain over a definition's own subtree
// rather than a URL prefix, which TestGroupUseIsMembershipScopedNotPrefix holds;
// wrapping the leaf says the same thing without depending on which version is
// pinned, and these are the routes that move money.
//
// mount is the address these routes answer at, because a recorded path has to be
// the path the router matches and a group has no single absolute prefix until a
// build resolves the tree. It is a claim, so it is CHECKED:
// TestMintRoutesMatchWhatIsServed compares the registry against the app's own
// declaration.
//
// Declare on a group whose caller is already resolved — PlatformOnly reads what
// TokenRequired set and only ever narrows.
func Mint(r *zip.Group, mount string) Money {
	return Money{Group: r.With(PlatformOnlyMW), mount: mount}
}

func (m Money) record(method, path string) {
	mintMu.Lock()
	mintRegistry[MintRoute{Method: method, Path: joinPath(m.mount, path)}] = struct{}{}
	mintMu.Unlock()
}

// Post declares a money operation. The four mutating verbs are here because a
// mint is a write; a read under the same group is declared with Get and gated
// the same way, and recording it would claim the surface is wider than it is.
func (m Money) Post[In, Out any](path string, fn zip.TypedHandler[In, Out], opts ...zip.OpOption) *zip.Operation[In, Out] {
	m.record(http.MethodPost, path)
	return m.Group.Post(path, fn, opts...)
}

func (m Money) Put[In, Out any](path string, fn zip.TypedHandler[In, Out], opts ...zip.OpOption) *zip.Operation[In, Out] {
	m.record(http.MethodPut, path)
	return m.Group.Put(path, fn, opts...)
}

func (m Money) Patch[In, Out any](path string, fn zip.TypedHandler[In, Out], opts ...zip.OpOption) *zip.Operation[In, Out] {
	m.record(http.MethodPatch, path)
	return m.Group.Patch(path, fn, opts...)
}

func (m Money) Delete[In, Out any](path string, fn zip.TypedHandler[In, Out], opts ...zip.OpOption) *zip.Operation[In, Out] {
	m.record(http.MethodDelete, path)
	return m.Group.Delete(path, fn, opts...)
}

// Raw declares a money route that is not a typed operation, and records it the
// same way. A money route invisible to the projections is a separate problem from
// a money route invisible to the registry, and this keeps the second from
// happening while the first is fixed.
func (m Money) Raw(method, path string, handlers ...zip.Handler) *zip.Group {
	if method != zip.MethodAll {
		m.record(method, path)
	}
	return m.Group.Raw(method, path, handlers...)
}

// MintRoutes is every money route declared through Money, sorted and
// deduplicated — the mint surface, derived from the declarations themselves
// rather than hand-listed by each consumer.
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

// joinPath mirrors the router's own group-path spelling, so a recorded path
// equals the path it actually matches.
func joinPath(prefix, path string) string {
	switch {
	case prefix == "":
		return path
	case path == "" || path == "/":
		return prefix
	case strings.HasSuffix(prefix, "/") && strings.HasPrefix(path, "/"):
		return prefix + strings.TrimPrefix(path, "/")
	case !strings.HasSuffix(prefix, "/") && !strings.HasPrefix(path, "/"):
		return prefix + "/" + path
	}
	return prefix + path
}
