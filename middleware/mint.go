// Copyright © 2026 Hanzo AI. MIT License.

package middleware

import (
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/zap-proto/zip"
)

// MintRoute is one money route: method and full path.
type MintRoute struct {
	Method string
	Path   string
}

var (
	mintMu       sync.Mutex
	mintRegistry = map[MintRoute]struct{}{}
)

// Money is a gated group that records each route declared through it.
type Money struct {
	*zip.Group
	mount string
}

// Mint gates r with PlatformOnly. mount is the address r is served at.
func Mint(r *zip.Group, mount string) Money {
	return Money{Group: r.With(PlatformOnlyMW), mount: mount}
}

func (m Money) record(method, path string) {
	mintMu.Lock()
	mintRegistry[MintRoute{Method: method, Path: joinPath(m.mount, path)}] = struct{}{}
	mintMu.Unlock()
}

// Post, Put, Patch and Delete declare a typed money route and record it.
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

// Raw declares an untyped money route and records it.
func (m Money) Raw(method, path string, handlers ...zip.Handler) *zip.Group {
	if method != zip.MethodAll {
		m.record(method, path)
	}
	return m.Group.Raw(method, path, handlers...)
}

// MintRoutes lists the recorded money routes, sorted.
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

// joinPath joins a group prefix and a leaf the way the router does.
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
