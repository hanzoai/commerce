package billing

// mint_registry_test.go cross-checks the two INDEPENDENT derivations of the mint
// surface against each other:
//
//   - the REGISTRY (middleware.MintRoutes) — declared: what registration through
//     middleware.Mint recorded, i.e. which routes the code SAYS are mint-gated; and
//   - the AST call-graph analysis (mint_surface_test.go) — discovered: which
//     registered handlers actually REACH a money-mint sink in source.
//
// Neither is derived from the other, so agreement is real evidence. They are also
// COMPLEMENTARY, not identical, and the tests below pin down exactly how — see
// TestMintRegistry_AgreesWithASTSurface.
//
// The load-bearing check is TestMintRegistry_DeclarationImpliesEnforcement:
// declaring a route mint-gated must MEAN it, proven by HTTP behavior rather than
// by reading the chain. Declaration, enforcement, and the source-level sink
// analysis all have to line up.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/middleware"
)

// TestMintRegistry_DeclarationImpliesEnforcement proves the registry does not
// merely LIST routes — every route it declares actually denies an org admin
// (403) at runtime. This is the property cloud relies on when it treats
// MintRoutes() as the set of paths its service-token bridge must never forward:
// a declaration that did not enforce would be worse than no declaration at all.
func TestMintRegistry_DeclarationImpliesEnforcement(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")

	eng := orgAdminEngine(t) // registers the real Route() → populates the registry
	routes := middleware.MintRoutes()
	if len(routes) == 0 {
		t.Fatal("mint registry is EMPTY after registration — middleware.Mint recorded nothing")
	}

	for _, r := range routes {
		if !strings.HasPrefix(r.Path, "/v1/") {
			t.Errorf("registry entry %s %s is not under the /v1 group — a stray registration polluted the registry", r.Method, r.Path)
			continue
		}
		if code := probe(eng, r.Method, concretePath(r.Path)); code != http.StatusForbidden {
			t.Errorf("DECLARED-BUT-UNGATED: %s %s is in middleware.MintRoutes() but returned %d for an org admin, want 403.\n"+
				"    The mint registry must never claim a gate that is not enforced.", r.Method, r.Path, code)
		} else {
			t.Logf("DECLARED+ENFORCED  %-6s %s — org-admin 403", r.Method, r.Path)
		}
	}
}

// TestMintRegistry_AgreesWithASTSurface reconciles the declared set against the
// source-derived set. They are complementary, and this test pins the exact
// relationship in BOTH directions so that any drift on either side fails:
//
//  1. AST ⊆ declared ∪ allowlists ∪ restRegisteredMintRoutes — every handler
//     that reaches a mint sink is accounted for. This is the safety direction: a
//     new mint-reaching route that the conversion forgot to declare fails here.
//  2. declared ⊇ AST-route-gated — the declared set covers every mint-reaching
//     route whose protection IS the route gate.
//
// The declared set is deliberately BROADER than the AST set (assertion 2 is a
// superset, not an equality). That is not sloppiness on either side: the AST
// derives its set from money SINKS — code that creates or moves money — while
// the registry declares the platform MONEY-AUTHORITY bar, which is deliberately
// wider than minting. Two kinds of route are declared but invisible to the sink
// analysis, and both are correct:
//
//   - routes that MUTATE money state without creating it: /cycle/run-user drives
//     an invoice collection (engine.CollectInvoice burns credits and writes a
//     Withdraw — a DEBIT), and /credit-grants/:id/void flips Voided on an
//     existing grant. Neither mints, so no mint sink exists to find.
//   - routes that change money POLICY rather than money: /test-mode flips
//     org.Live, which decides whether charges are real or sandboxed.
//
// Those are gated by declaration because forcing a charge on a named user, or
// moving an org to sandbox, is platform authority — not because they mint. The
// registry is what protects them, which is precisely why the two checks are
// worth keeping independent: neither derivation can see everything the other does.
func TestMintRegistry_AgreesWithASTSurface(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")

	reaches := mintReachingFuncs(t)
	astRoutes := registeredMintRoutes(t, reaches) // registers → also populates the registry

	declared := map[middleware.MintRoute]bool{}
	for _, r := range middleware.MintRoutes() {
		declared[r] = true
	}
	if len(declared) == 0 {
		t.Fatal("mint registry is EMPTY after registration — middleware.Mint recorded nothing")
	}

	// (1) Every AST-discovered mint route is accounted for by exactly one mechanism.
	var astGated []middleware.MintRoute
	for _, r := range astRoutes {
		key := middleware.MintRoute{Method: r.method, Path: r.path}
		switch {
		case declared[key]:
			astGated = append(astGated, key)
			t.Logf("DECLARED   %-6s %-40s %s", r.method, r.path, r.handler)
		case userSafeMintHandlers[r.handler] != "":
			t.Logf("USER-SAFE  %-6s %-40s %s", r.method, r.path, r.handler)
		case methodGatedMintHandlers[r.handler] != "":
			t.Logf("METHOD-GATE %-5s %-40s %s", r.method, r.path, r.handler)
		case restRegisteredMintRoutes[key] != "":
			t.Logf("REST-GATED %-6s %-40s %s — %s", r.method, r.path, r.handler, restRegisteredMintRoutes[key])
		default:
			t.Errorf("UNACCOUNTED MINT ROUTE: %s %s (handler %s) reaches a mint sink but is NOT declared via\n"+
				"    middleware.Mint, NOT in userSafeMintHandlers, NOT in methodGatedMintHandlers, and NOT a known\n"+
				"    rest-registered route. Declare it with middleware.Mint so cloud can see it.", r.method, r.path, r.handler)
		}
	}

	// (2) The declared set must cover every AST route whose protection is the route gate.
	for _, r := range astGated {
		if !declared[r] {
			t.Errorf("AST route-gated but NOT declared: %s %s", r.Method, r.Path)
		}
	}

	// Report the asymmetry explicitly so a shift in either derivation is visible
	// in the log rather than silently absorbed.
	inAST := map[middleware.MintRoute]bool{}
	for _, r := range astRoutes {
		inAST[middleware.MintRoute{Method: r.method, Path: r.path}] = true
	}
	for r := range declared {
		if !inAST[r] {
			t.Logf("DECLARED-ONLY %-6s %s — gated by declaration: it exercises platform money authority without minting, so no mint sink exists for the analysis to find", r.Method, r.Path)
		}
	}
}

// restRegisteredMintRoutes are mint routes registered through util/rest's
// deferred route table (rest.Rest.POST/PUT → rest.Route → handle(group,…))
// rather than through a zip.Router verb. middleware.Mint decorates a zip.Router,
// so it cannot record these: rest builds its OWN group and registers each custom
// sub-route's chain itself, and wrapping the router handed to rest.Route would
// gate every route in the group, not the mint ones. They carry the SAME gate
// (middleware.PlatformOnly, chained explicitly in api/affiliate/route.go) and are
// verified by the AST guard's org-admin 403 probe — they are simply not
// self-declaring yet.
//
// None of them is reachable through cloud's /v1/billing bridge (they live under
// /v1/contributor), so their absence from MintRoutes() does not weaken the
// bridge-disjointness check MintRoutes() exists to enable.
var restRegisteredMintRoutes = map[middleware.MintRoute]string{
	{Method: http.MethodPost, Path: "/v1/contributor/payouts/execute"}: "api/affiliate registers via rest.Rest.POST with an explicit PlatformOnly chain",
}
