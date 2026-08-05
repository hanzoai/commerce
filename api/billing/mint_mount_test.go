package billing

import (
	"strings"
	"testing"

	"github.com/hanzoai/commerce/middleware"
	"github.com/zap-proto/zip"
)

// TestMintRoutesMatchWhatIsServed is what makes a STATED mount address safe to
// state.
//
// Mint records each money route at mintMountPath joined with the leaf, because a
// router cannot report its own absolute address — zip composes definitions, so
// the same one can be included at more than one site, and there is no single
// prefix to read until a build resolves the tree. The previous code read
// r.Fiber().(*fiber.Group).Prefix and got an answer only because fiber flattened
// a group into one prefix at declaration.
//
// So the address is a claim by the code that mounts these routes. This checks the
// claim against the app's own declaration: every path MintRoutes() reports must be
// a path the built program actually serves. A wrong constant — a stale /v1, a
// renamed group, a second mount — fails here rather than silently producing a mint
// surface that names addresses nothing answers, which is exactly the surface
// cloud's forwardable allowlist is diffed against.
func TestMintRoutesMatchWhatIsServed(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	routeMintSurface(app.Group("/v1"))

	served := map[string]bool{}
	for _, r := range app.Declaration().Routes {
		served[r.Method+" "+r.Pattern] = true
	}

	mint := middleware.MintRoutes()
	if len(mint) == 0 {
		t.Fatal("no mint routes recorded; the surface this guards would be vacuously empty")
	}

	var missing []string
	for _, m := range mint {
		if !strings.HasPrefix(m.Path, "/v1/") {
			t.Errorf("mint route %s %s is not an absolute served address", m.Method, m.Path)
			continue
		}
		if !served[m.Method+" "+m.Path] {
			missing = append(missing, m.Method+" "+m.Path)
		}
	}
	if len(missing) > 0 {
		var have []string
		for k := range served {
			if strings.Contains(k, "/billing") {
				have = append(have, k)
			}
		}
		t.Errorf("MintRoutes() names %d address(es) the program does not serve:\n  %s\n"+
			"served under /billing:\n  %s\n\n"+
			"mintMountPath in handlers.go states where these routes are mounted; it and the\n"+
			"actual mount have diverged.", len(missing), strings.Join(missing, "\n  "), strings.Join(have, "\n  "))
	}
}
