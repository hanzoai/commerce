package billing

// mint_surface_test.go is the ANTI-REGRESSION guard for C1 (the org-owner
// free-balance hole). Instead of hard-coding a list of mint routes — which is
// exactly what let three more mint surfaces (/zap billing.deposit,
// /allotment/grant, /contributor/payouts/execute) slip past the first fix — it
// ENUMERATES the actual mint surface from SOURCE and asserts every reachable
// mint route is either gated (org-admin → 403) or provably user-safe.
//
// How it works:
//  1. AST-scan the api/billing and api/affiliate packages for every function
//     that REACHES a money-mint SINK — a `x.Type = transaction.Deposit` write, a
//     call to allotment.Grant, or a call to the contributor payout executor —
//     following same-package calls transitively. This set is derived from the
//     code, so a NEW handler that mints is detected automatically.
//  2. Mount the REAL Route() for both packages and enumerate registered routes.
//  3. For every registered route whose handler reaches a sink, assert it is
//     route-gated (probe as an org admin → 403), or method-gated inside the
//     handler (ZapDispatch), or on the explicit userSafe allowlist WITH a reason.
//
// A new ungated mint route makes this test FAIL — the guard the C1 miss proved we
// need. The allowlist is checked back against the source set, so a stale entry
// (a handler that no longer mints) also fails.

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/api/affiliate"
	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
)

// ── mint-sink source analysis ───────────────────────────────────────────────

// pkgDirs are the packages whose route handlers can mint. Relative to this test
// file's directory (api/billing) at `go test` runtime.
var mintPkgDirs = []string{".", "../affiliate"}

// mintReachingFuncs returns the set of function short-names (across the analyzed
// packages) whose bodies reach a money-mint sink, following same-package calls
// transitively. Derived from source so a new mint handler is auto-detected.
func mintReachingFuncs(t *testing.T) map[string]bool {
	t.Helper()

	rawSink := map[string]bool{}          // func → directly contains a sink
	calls := map[string]map[string]bool{} // func → same-package funcs it calls

	for _, dir := range mintPkgDirs {
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, dir, func(fi fs.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", dir, err)
		}
		for _, pkg := range pkgs {
			for _, file := range pkg.Files {
				imports := importAliases(file)
				for _, decl := range file.Decls {
					fn, ok := decl.(*ast.FuncDecl)
					if !ok || fn.Body == nil {
						continue
					}
					name := fn.Name.Name
					if _, seen := calls[name]; !seen {
						calls[name] = map[string]bool{}
					}
					ast.Inspect(fn.Body, func(n ast.Node) bool {
						switch x := n.(type) {
						case *ast.AssignStmt:
							// x.Type = transaction.Deposit  (a deposit-WRITE, not a
							// `== transaction.Deposit` read comparison — that is a
							// BinaryExpr, never an AssignStmt RHS).
							for _, rhs := range x.Rhs {
								if isSelector(rhs, imports, "models/transaction", "Deposit") {
									rawSink[name] = true
								}
							}
							// x.DestinationKind = "iam-user"  (RED's exact criterion) —
							// mints spendable balance to an IAM user. Precisely the
							// DESTINATION field, so a Withdraw's SourceKind="iam-user"
							// (a debit) is NOT flagged.
							for i, lhs := range x.Lhs {
								if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "DestinationKind" &&
									i < len(x.Rhs) && isStringLit(x.Rhs[i], "iam-user") {
									rawSink[name] = true
								}
							}
						case *ast.CallExpr:
							// allotment.Grant(...) — mints included monthly credit.
							if isSelectorCall(x, imports, "billing/allotment", "Grant") {
								rawSink[name] = true
							}
							// <payout cron>.Payout(...) — disburses treasury.
							if isPayoutExecutorCall(x, imports) {
								rawSink[name] = true
							}
							// same-package call by bare identifier → call-graph edge.
							if id, ok := x.Fun.(*ast.Ident); ok {
								calls[name][id.Name] = true
							}
						}
						return true
					})
				}
			}
		}
	}

	// Transitive closure: a func reaches a sink if it is a raw sink or calls a
	// same-package func that reaches a sink.
	reaches := map[string]bool{}
	for f := range rawSink {
		reaches[f] = true
	}
	for changed := true; changed; {
		changed = false
		for f, callees := range calls {
			if reaches[f] {
				continue
			}
			for callee := range callees {
				if reaches[callee] {
					reaches[f] = true
					changed = true
					break
				}
			}
		}
	}
	return reaches
}

// importAliases maps a file's import local-name → import path.
func importAliases(file *ast.File) map[string]string {
	m := map[string]string{}
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		name := path[strings.LastIndex(path, "/")+1:]
		if imp.Name != nil {
			name = imp.Name.Name
		}
		m[name] = path
	}
	return m
}

// isStringLit reports whether e is the string literal `want`.
func isStringLit(e ast.Expr, want string) bool {
	lit, ok := e.(*ast.BasicLit)
	return ok && lit.Kind == token.STRING && strings.Trim(lit.Value, "`\"") == want
}

// isSelector reports whether e is `alias.sel` where alias imports a path ending
// in pkgSuffix.
func isSelector(e ast.Expr, imports map[string]string, pkgSuffix, sel string) bool {
	s, ok := e.(*ast.SelectorExpr)
	if !ok || s.Sel.Name != sel {
		return false
	}
	id, ok := s.X.(*ast.Ident)
	if !ok {
		return false
	}
	return strings.HasSuffix(imports[id.Name], pkgSuffix)
}

// isSelectorCall reports whether call is `alias.sel(...)` where alias imports a
// path ending in pkgSuffix.
func isSelectorCall(call *ast.CallExpr, imports map[string]string, pkgSuffix, sel string) bool {
	return isSelector(call.Fun, imports, pkgSuffix, sel)
}

// isPayoutExecutorCall reports whether call disburses the treasury: a `.Payout`
// call on an import whose path is under cron/payout (the OSS-contributor executor).
func isPayoutExecutorCall(call *ast.CallExpr, imports map[string]string) bool {
	s, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || s.Sel.Name != "Payout" {
		return false
	}
	id, ok := s.X.(*ast.Ident)
	if !ok {
		return false
	}
	return strings.Contains(imports[id.Name], "cron/payout")
}

// ── the guard ───────────────────────────────────────────────────────────────

// userSafeMintHandlers are registered handlers that DO reach a mint sink but are
// provably safe for a self-service (org-admin) caller — each with the reason it
// need not carry the platform-only gate. A stale entry (handler no longer mints)
// is flagged by the guard, and a NEW mint handler NOT listed here (and not
// route-gated) FAILS the guard.
var userSafeMintHandlers = map[string]string{
	"GrantStarterCredit":    "fixed StarterCreditCents to the caller's OWN org key (orgBillingKey); tag-idempotent one-per-subject",
	"PostMyWelcome":         "fixed StarterCreditCents to the caller's OWN org key; deterministic-key idempotent (TestWelcome_ConcurrentGrantsCreditOnce)",
	"Topup":                 "credits ONLY the amount the caller's own saved card was charged (money-in == credit); own subject",
	"TopupWithToken":        "credits ONLY the amount the caller's own card nonce was charged (money-in == credit); own subject",
	"GrantAllotment":        "amount is clamped to the caller's REAL subscription via planForGrant unless MayMintMoney (TestAllotment_OrgAdminCannotInflatePlan)",
	"RunAllotments":         "per-user amount is subscription-derived in grantOrgAllotments; NO client-supplied amount or plan",
	"HandleProviderWebhook": "unauthenticated by design — trust anchor is the per-provider signature, not a commerce token; not an org-admin surface",
}

// methodGatedMintHandlers reach a sink but gate it INSIDE the handler per-method
// (not via route middleware), each verified by a named companion test.
var methodGatedMintHandlers = map[string]string{
	"ZapDispatch": "mint methods (billing.deposit) gated per-method on middleware.MayMintMoney (TestZapDeposit_OrgAdminDenied / TestZapReads_OrgAdminNotBlocked)",
}

type mintRoute struct{ method, path, handler string }

// TestMintSurface_EveryMintRouteGatedOrProvablyUserSafe is the enumeration guard.
func TestMintSurface_EveryMintRouteGatedOrProvablyUserSafe(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")

	reaches := mintReachingFuncs(t)

	// Detector self-check: the known-critical mint functions MUST be flagged, or
	// the enumeration is silently under-detecting (which is the whole failure mode
	// we are guarding against).
	for _, must := range []string{"Deposit", "Refund", "GrantAllotment", "zapDeposit", "ZapDispatch", "PostMyWelcome", "executePayouts"} {
		if !reaches[must] {
			t.Fatalf("mint-sink detector did NOT flag %q — the source enumeration is broken; fix the detector before trusting this guard", must)
		}
	}

	// Enumerate every registered route whose handler reaches a mint sink.
	routes := registeredMintRoutes(t, reaches)
	if len(routes) == 0 {
		t.Fatal("no mint routes discovered — routing/enumeration is broken")
	}

	// Cross-check each against gating / allowlist.
	orgAdmin := orgAdminEngine(t)
	for _, r := range routes {
		if reason, ok := userSafeMintHandlers[r.handler]; ok {
			t.Logf("USER-SAFE  %-6s %-40s %s — %s", r.method, r.path, r.handler, reason)
			continue
		}
		if reason, ok := methodGatedMintHandlers[r.handler]; ok {
			t.Logf("METHOD-GATE %-5s %-40s %s — %s", r.method, r.path, r.handler, reason)
			continue
		}
		// Must be route-gated: an org admin must be denied (403) BEFORE the handler.
		code := probe(orgAdmin, r.method, concretePath(r.path))
		if code != http.StatusForbidden {
			t.Errorf("UNGATED MINT ROUTE: %s %s (handler %s) returned %d for an org admin, want 403.\n"+
				"    Add middleware.PlatformOnly() to the route, gate it inside the handler on middleware.MayMintMoney,\n"+
				"    or justify it in userSafeMintHandlers with a concrete reason.", r.method, r.path, r.handler, code)
		} else {
			t.Logf("GATED      %-6s %-40s %s — org-admin 403", r.method, r.path, r.handler)
		}
	}

	// No stale allowlist entries: everything allowlisted must still reach a sink.
	for h := range userSafeMintHandlers {
		if !reaches[h] {
			t.Errorf("userSafeMintHandlers[%q] no longer reaches a mint sink — remove the stale allowlist entry", h)
		}
	}
	for h := range methodGatedMintHandlers {
		if !reaches[h] {
			t.Errorf("methodGatedMintHandlers[%q] no longer reaches a mint sink — remove the stale allowlist entry", h)
		}
	}
}

// registeredMintRoutes mounts the real billing + affiliate Route()s and returns
// every registered route whose terminal handler reaches a mint sink.
func registeredMintRoutes(t *testing.T, reaches map[string]bool) []mintRoute {
	t.Helper()
	gin.SetMode(gin.TestMode)
	eng := gin.New()
	v1 := eng.Group("/v1")
	Route(v1)
	affiliate.Route(v1)

	var out []mintRoute
	for _, ri := range eng.Routes() {
		short := ri.Handler[strings.LastIndex(ri.Handler, ".")+1:]
		if reaches[short] {
			out = append(out, mintRoute{ri.Method, ri.Path, short})
		}
	}
	return out
}

// orgAdminEngine mounts billing + affiliate behind a seed that mints an ORG-admin
// identity (org-level isAdmin, isGlobalAdmin=false) — the exact C1 adversary.
func orgAdminEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	eng := gin.New()
	eng.Use(func(c *gin.Context) {
		c.Set("iam_authenticated", true)
		c.Set("permissions", bit.Field(permission.Admin|permission.Live))
		c.Set("iam_claims", &auth.IAMClaims{Owner: "acme", IsAdmin: true})
		c.Next()
	})
	v1 := eng.Group("/v1")
	Route(v1)
	affiliate.Route(v1)
	return eng
}

func probe(eng *gin.Engine, method, path string) int {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	eng.ServeHTTP(w, req)
	return w.Code
}

// concretePath replaces gin `:param` / `*param` segments with a literal so the
// route resolves during probing.
func concretePath(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		if strings.HasPrefix(s, ":") || strings.HasPrefix(s, "*") {
			segs[i] = "x"
		}
	}
	return strings.Join(segs, "/")
}
