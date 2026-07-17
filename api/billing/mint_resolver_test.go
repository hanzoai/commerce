package billing

// mint_resolver_test.go pins the CALL SHAPES the sink analysis can follow.
//
// The analyzer is a small hand-rolled resolver, and its failure mode is SILENT:
// a shape it cannot resolve yields no edge, so a minting handler simply never
// appears in the surface and every guard downstream passes. That is invisible
// from the real routes — which is exactly how the husd handlers hid — so the
// shapes are pinned here against synthetic source instead of waiting for a real
// handler to be written in a shape the resolver happens to miss.
//
// Each case reaches the SAME sink through the same service; only the calling
// shape differs. A case that stops being caught is a regression in reachability,
// not a change in the code under test.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// resolverFixture is a two-package program: a money service whose method mints,
// and a caller that reaches it through some shape. `svcpkg` deliberately mirrors
// the husdledger shape (a package-level Default() returning *Service).
const resolverSvcSrc = `package svcpkg

import "github.com/hanzoai/commerce/mintauth"

type Service struct{}

// Settle is the sink: it asserts mint authority.
func (s *Service) Settle(ctx int) error {
	_ = mintauth.WithAuthorized(nil)
	return nil
}

// Readonly must NOT be a sink — it is the over-approximation canary.
func (s *Service) Readonly() string { return "" }

func Default() *Service { return &Service{} }
`

// parseResolverCase builds pkg → files for the service package plus a caller.
func parseResolverCase(t *testing.T, callerSrc string) map[string][]*ast.File {
	t.Helper()
	fset := token.NewFileSet()
	svc, err := parser.ParseFile(fset, "svc.go", resolverSvcSrc, 0)
	if err != nil {
		t.Fatalf("parse service fixture: %v", err)
	}
	caller, err := parser.ParseFile(fset, "caller.go", callerSrc, 0)
	if err != nil {
		t.Fatalf("parse caller fixture: %v", err)
	}
	return map[string][]*ast.File{
		"billing/svcpkg": {svc},
		"api/callerpkg":  {caller},
	}
}

func TestResolver_CallShapes(t *testing.T) {
	// The import path must be the module-relative one the resolver keys on.
	const imports = `import (
	svcpkg "github.com/hanzoai/commerce/billing/svcpkg"
)`

	cases := []struct {
		name string
		src  string
		want bool
	}{{
		name: "local from constructor",
		src: `package callerpkg
` + imports + `
func h() { svc := svcpkg.Default(); _ = svc.Settle(0) }`,
		want: true,
	}, {
		name: "chained on constructor",
		src: `package callerpkg
` + imports + `
func h() { _ = svcpkg.Default().Settle(0) }`,
		want: true,
	}, {
		name: "service as parameter",
		src: `package callerpkg
` + imports + `
func h(svc *svcpkg.Service) { _ = svc.Settle(0) }`,
		want: true,
	}, {
		name: "embedded promotion",
		src: `package callerpkg
` + imports + `
type wrap struct{ *svcpkg.Service }
func h(w *wrap) { _ = w.Settle(0) }`,
		want: true,
	}, {
		name: "var declaration",
		src: `package callerpkg
` + imports + `
func h() { var svc = svcpkg.Default(); _ = svc.Settle(0) }`,
		want: true,
	}, {
		name: "typed var declaration",
		src: `package callerpkg
` + imports + `
func h() { var svc *svcpkg.Service; _ = svc.Settle(0) }`,
		want: true,
	}, {
		name: "package-level var",
		src: `package callerpkg
` + imports + `
var svc = svcpkg.Default()
func h() { _ = svc.Settle(0) }`,
		want: true,
	}, {
		name: "method value",
		src: `package callerpkg
` + imports + `
func h() { svc := svcpkg.Default(); m := svc.Settle; _ = m(0) }`,
		want: true,
	}, {
		name: "struct field",
		src: `package callerpkg
` + imports + `
type holder struct{ svc *svcpkg.Service }
func h(x *holder) { _ = x.svc.Settle(0) }`,
		want: true,
	}, {
		name: "transitively via a same-package helper",
		src: `package callerpkg
` + imports + `
func helper(svc *svcpkg.Service) { _ = svc.Settle(0) }
func h() { helper(svcpkg.Default()) }`,
		want: true,
	}, {
		// The canary: reaching a NON-minting method of a minting service must not
		// flag the caller. A resolver that flagged this would be marking whole
		// packages, and every "GATED" line in the surface guard would be noise.
		name: "read-only method is not a sink",
		src: `package callerpkg
` + imports + `
func h() { svc := svcpkg.Default(); _ = svc.Readonly() }`,
		want: false,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reaches := analyzeFiles(t, parseResolverCase(t, tc.src))
			if got := reaches["api/callerpkg.h"]; got != tc.want {
				t.Errorf("reaches[api/callerpkg.h] = %v, want %v — the resolver cannot follow this shape,\n"+
					"    so a handler written this way would mint invisibly and every downstream guard would pass.", got, tc.want)
			}
		})
	}
}

// TestResolver_KnownLimits documents, by asserting them, the shapes the resolver
// does NOT follow. These are honest gaps, not bugs to be papered over: each one
// is a way a mint could hide, so they are written down and asserted rather than
// left to be discovered. If one starts passing, delete it from here — the
// resolver got stronger and the limit is no longer real.
func TestResolver_KnownLimits(t *testing.T) {
	const imports = `import (
	svcpkg "github.com/hanzoai/commerce/billing/svcpkg"
)`

	cases := []struct{ name, why, src string }{{
		name: "interface dispatch",
		why:  "the dynamic type is not known without full type checking; husdindex.Ledger.Credit is reached this way and is covered instead by a sink at the concrete implementation",
		src: `package callerpkg
` + imports + `
type settler interface{ Settle(ctx int) error }
func h(s settler) { _ = s.Settle(0) }
var _ = svcpkg.Default`,
	}, {
		name: "closure captured into a variable",
		why:  "a func literal assigned to a var and called through it is not tracked; the literal's BODY is still walked, so a sink inside it is attributed to the enclosing func",
		src: `package callerpkg
` + imports + `
func h(svc *svcpkg.Service) { var f func(); f = func() { g(svc) }; f() }
func g(svc *svcpkg.Service) {}`,
	}, {
		name: "value returned through an interface",
		why:  "the concrete type behind an interface-typed result is unknown",
		src: `package callerpkg
` + imports + `
type settler interface{ Settle(ctx int) error }
func mk() settler { return svcpkg.Default() }
func h() { _ = mk().Settle(0) }`,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reaches := analyzeFiles(t, parseResolverCase(t, tc.src))
			if reaches["api/callerpkg.h"] {
				t.Errorf("resolver now FOLLOWS %q (%s) — that is an improvement; delete this case from KnownLimits", tc.name, tc.why)
			} else {
				t.Logf("KNOWN LIMIT: %s — %s", tc.name, tc.why)
			}
		})
	}
}
