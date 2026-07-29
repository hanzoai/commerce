package billing

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/zap-proto/fiber/v3"
	"github.com/zap-proto/zip"
)

// ── zip-native route introspection ───────────────────────────────────────────
//
// zip/fiber does not expose gin's engine.Routes() (a route list carrying the
// bound handler func for reflection). recordingRouter is the replacement: it
// satisfies zip.Router, DELEGATES every registration to a real backing app (so
// the same app also serves), and records (method, full-path, terminal-handler
// name) for the route-table guards. Path joining mirrors fiber's getGroupPath
// so a recorded path equals the path fiber actually routes.

type routeRec struct{ method, path, handler string }

type recordingRouter struct {
	inner  zip.Router
	prefix string
	rec    *[]routeRec
}

// newRecordingRouter wraps app's root router and records into a fresh slice.
func newRecordingRouter(app *zip.App) *recordingRouter {
	return &recordingRouter{inner: app.Group(""), rec: &[]routeRec{}}
}

// joinGroupPath reproduces fiber's getGroupPath: a leaf/segment without a
// leading slash gets one, and the parent's trailing slash is trimmed.
func joinGroupPath(prefix, path string) string {
	if path == "" {
		return prefix
	}
	if path[0] != '/' {
		path = "/" + path
	}
	return strings.TrimRight(prefix, "/") + path
}

// handlerFullName is the fully-qualified name of the func a route is bound to
// (e.g. ".../api/billing.ListTransactions") — the zip-native analog of gin's
// RouteInfo.Handler.
func handlerFullName(h zip.Handler) string {
	return runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name()
}

func (r *recordingRouter) record(method, path string, hs []zip.Handler) {
	if len(hs) == 0 {
		return
	}
	*r.rec = append(*r.rec, routeRec{method, joinGroupPath(r.prefix, path), handlerFullName(hs[len(hs)-1])})
}

func (r *recordingRouter) Use(handlers ...zip.Handler) zip.Router { r.inner.Use(handlers...); return r }

func (r *recordingRouter) Get(p string, h ...zip.Handler) zip.Router {
	r.record(http.MethodGet, p, h)
	r.inner.Get(p, h...)
	return r
}
func (r *recordingRouter) Post(p string, h ...zip.Handler) zip.Router {
	r.record(http.MethodPost, p, h)
	r.inner.Post(p, h...)
	return r
}
func (r *recordingRouter) Put(p string, h ...zip.Handler) zip.Router {
	r.record(http.MethodPut, p, h)
	r.inner.Put(p, h...)
	return r
}
func (r *recordingRouter) Patch(p string, h ...zip.Handler) zip.Router {
	r.record(http.MethodPatch, p, h)
	r.inner.Patch(p, h...)
	return r
}
func (r *recordingRouter) Delete(p string, h ...zip.Handler) zip.Router {
	r.record(http.MethodDelete, p, h)
	r.inner.Delete(p, h...)
	return r
}
func (r *recordingRouter) Head(p string, h ...zip.Handler) zip.Router {
	r.record(http.MethodHead, p, h)
	r.inner.Head(p, h...)
	return r
}
func (r *recordingRouter) Options(p string, h ...zip.Handler) zip.Router {
	r.record(http.MethodOptions, p, h)
	r.inner.Options(p, h...)
	return r
}
func (r *recordingRouter) All(p string, h ...zip.Handler) zip.Router {
	r.record("ALL", p, h)
	r.inner.All(p, h...)
	return r
}
func (r *recordingRouter) Group(prefix string, handlers ...zip.Handler) zip.Router {
	return &recordingRouter{inner: r.inner.Group(prefix, handlers...), prefix: joinGroupPath(r.prefix, prefix), rec: r.rec}
}
func (r *recordingRouter) Fiber() fiber.Router { return r.inner.Fiber() }

// OpScope answers where a TYPED op declared on this recorder lands: the inner
// router's scope, untouched. This decorator only records, so it has no
// middleware to fold in and nothing to change about where an op goes — but zip
// asks every Router, so every Router answers.
func (r *recordingRouter) OpScope() zip.OpScope { return r.inner.OpScope() }

// ── route registration guards ────────────────────────────────────────────────

// mountRoutes mounts the billing Route() onto a bare zip app under the
// production "/v1" api prefix and returns the app plus the recorded route table.
// A recordingRouter satisfies zip.Router, so this exercises the real
// registration code path without router.New()'s global http.Handle side effects.
func mountRoutes(t *testing.T) (*zip.App, []routeRec) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	rr := newRecordingRouter(app)
	Route(rr.Group("/v1"))
	return app, *rr.rec
}

// TestTransactionsRouteRegistered proves GET /v1/billing/transactions is routed
// and bound to ListTransactions. Regression guard: the handler existed but was
// never registered, so the billing dashboard's Transactions tab got a 404.
func TestTransactionsRouteRegistered(t *testing.T) {
	_, routes := mountRoutes(t)

	const wantMethod = http.MethodGet
	const wantPath = "/v1/billing/transactions"

	var found *routeRec
	for i := range routes {
		if routes[i].method == wantMethod && routes[i].path == wantPath {
			found = &routes[i]
			break
		}
	}

	if found == nil {
		t.Fatalf("route %s %s is not registered (the defect): handler ListTransactions exists but was never routed", wantMethod, wantPath)
	}

	if !strings.HasSuffix(found.handler, ".ListTransactions") {
		t.Fatalf("route %s %s resolves to %q, want it bound to ListTransactions", wantMethod, wantPath, found.handler)
	}
}

// TestTransactionsRouteNot404 confirms the path actually resolves at request
// time: an unauthenticated GET must reach the middleware/handler chain, never
// gin's "404 no route". (Auth makes it 401/400 here — the point is: not 404.)
func TestTransactionsRouteNot404(t *testing.T) {
	engine, _ := mountRoutes(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/billing/transactions", nil)
	resp, terr := engine.Fiber().Test(req)
	if terr != nil {
		t.Fatalf("Test: %v", terr)
	}

	if resp.StatusCode == http.StatusNotFound {
		t.Fatalf("GET /v1/billing/transactions returned 404 (no route); the route is not wired")
	}
}
