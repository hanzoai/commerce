package billing

// mint_surface_test.go is the ANTI-REGRESSION guard for C1 (the org-owner
// free-balance hole). Instead of hard-coding a list of mint routes — which is
// exactly what let three more mint surfaces (/zap billing.deposit,
// /allotment/grant, /contributor/payouts/execute) slip past the first fix — it
// ENUMERATES the actual mint surface from SOURCE and asserts every reachable
// mint route is either gated (org-admin → 403) or provably user-safe.
//
// How it works:
//  1. AST-scan the /v1 ledger route packages AND the money-engine packages they
//     call for every function that REACHES a money SINK — a mint-authority
//     assertion (mintauth.WithAuthorized), an on-chain token movement, a ledger
//     credit projection, a `x.Type = transaction.Deposit` write, an
//     allotment.Grant, the contributor payout executor, or the creation of a
//     money model (a credit grant, a payout) — following calls transitively and
//     ACROSS packages. This set is derived from the code, so a new handler that
//     mints is detected automatically.
//  2. Register the REAL Route() for those packages and enumerate registered routes.
//  3. For every registered route whose handler reaches a sink, assert it is
//     route-gated (probe as an org admin → 403), or method-gated inside the
//     handler (ZapDispatch), or on the explicit userSafe allowlist WITH a reason.
//
// A new ungated mint route makes this test FAIL — the guard the C1 miss proved we
// need. The allowlist is checked back against the source set, so a stale entry
// (a handler that no longer mints) also fails.
//
// Cross-package resolution is the whole ballgame, and its absence was a real
// hole: a handler that mints by CALLING a service (husdledger.Default().Settle)
// rather than by writing a ledger row itself was invisible, because the call
// graph followed only same-package calls by bare identifier and the engine
// packages were never even parsed. /husd/sync, /husd/settle and /husd/migrate
// were all mint-gated in production and completely unseen by this guard — their
// gates could have been deleted with the whole suite staying green.
//
// This guard is deliberately INDEPENDENT of middleware.MintRoutes(): it sees a
// route by what its handler DOES, never by how it was registered. Deriving it
// from the registry would be self-referential — un-gating a route would remove it
// from the registry and a registry-derived check would simply stop looking at it,
// which is exactly the blindness this exists to catch. mint_registry_test.go
// reconciles the two derivations; agreement is evidence only while they stay
// independent.

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/api/affiliate"
	transactionApi "github.com/hanzoai/commerce/api/transaction"
	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/demo/tokentransaction"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/wallet"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/rest"
)

// routeMintSurface registers on v1 the FULL /v1 ledger surface a mint could
// reach — billing + affiliate + the generic transaction router (POST
// /v1/transaction is C1-b) + the wallet and demo-tokentransaction REST routers.
// It is the ONE registration shared by the enumeration and the org-admin probe
// engine, so the guard sees the same routes api.Route(/v1) registers (minus the
// full-bootstrap bundle). Adding a mint route anywhere here brings it under the
// guard automatically. The per-router auth (tokenRequired/adminRequired) matches
// api.Route so an org-admin probe reproduces production reachability.
func routeMintSurface(v1 zip.Router) {
	tokenRequired := middleware.TokenRequired(permission.Admin)
	adminRequired := middleware.TokenRequired(permission.Admin)

	Route(v1)                // billing
	affiliate.Route(v1)      // affiliate payouts
	transactionApi.Route(v1) // POST /v1/transaction (generic ledger create — C1-b)
	rest.New(wallet.Wallet{}).Route(v1, adminRequired)
	rest.New(tokentransaction.Transaction{}).Route(v1, tokenRequired)
}

// ── mint-sink source analysis ───────────────────────────────────────────────

// modulePath is this module's import path and analysisPkg is THIS package's path
// within it — the anchor mintPkgDirs are written relative to. Together they are
// the ONE mapping between "a directory this test parses" and "the import path
// other packages name it by", so the scan set and the call resolver cannot drift.
const (
	modulePath  = "github.com/hanzoai/commerce"
	analysisPkg = "api/billing"
)

// mintPkgDirs are the packages a route handler can reach a money sink through,
// relative to this test file's directory (api/billing) at `go test` runtime.
// Two layers, and BOTH are needed:
//
//   - the /v1 ledger ROUTE surface (billing + affiliate + the generic transaction
//     router + the demo/account routers), so a mint that moves to a sibling
//     package is still caught; and
//   - the money ENGINE packages behind it. A handler mints by CALLING a service
//     (husdledger.Default().Settle(...)) at least as often as by writing a ledger
//     row itself. With the engine unscanned such a call just ends at a name the
//     graph has never heard of, and the mint is invisible — which is exactly how
//     /husd/sync, /husd/settle and /husd/migrate stayed unseen.
var mintPkgDirs = []string{
	".", "../affiliate", "../transaction", "../../demo/tokentransaction", "../account",
	"../../billing/husdledger",    // chain-backed credit ledger: mint / settle / migrate
	"../../billing/husdindex",     // on-chain transfer → ledger credit projection
	"../../treasury",              // the treasury-signed on-chain mint
	"../../billing/engine",        // customer-balance adjustment (the credit-direction write)
	"../../billing/depositledger", // crypto deposit → ledger credit (the watcher's write half)
	"../../billing/depositwatch",  // …and the policy that decides when it fires
}

// pkgPath maps a scan dir to the module-relative import path other packages name
// it by (mintPkgDirs are written relative to analysisPkg).
func pkgPath(dir string) string { return path.Clean(path.Join(analysisPkg, dir)) }

// inModule reports importPath's module-relative path, or "" when the import is
// outside this module (nothing outside it is ever a node in the graph).
func inModule(importPath string) string {
	if s := strings.TrimPrefix(importPath, modulePath+"/"); s != importPath {
		return s
	}
	return ""
}

// qualify names a func by the package that declares it — the node key of the
// call graph. Short names alone are NOT a safe key across packages: `Create`
// means api/transaction.Create (a mint) in one package and `x.Create()` (a plain
// datastore persist) in a dozen others, and merging them by name would smear
// mint-ness across the whole repo.
func qualify(pkg, name string) string { return pkg + "." + name }

// typeRef names a type by the package that declares it. The zero value means
// "unresolved", which is always safe: it yields no edge.
type typeRef struct{ pkg, name string }

// pkgSyms is one scanned package's symbol table — the minimum needed to answer
// "what package does the receiver of x.M(...) live in".
type pkgSyms struct {
	types   map[string]bool               // type names declared here
	fields  map[string]map[string]typeRef // struct → field → field's type
	returns map[string]typeRef            // func/method name → its first result type
	funcs   map[string]bool               // func/method names declared here
	embeds  map[string][]typeRef          // struct → the types it EMBEDS, for promotion
	globals map[string]typeRef            // package-level var → its type
}

// analyzer resolves selector calls across the scanned package set and collects
// the money-sink call graph.
type analyzer struct {
	syms    map[string]*pkgSyms        // pkg → symbols
	rawSink map[string]bool            // qualified func → contains a sink outright
	calls   map[string]map[string]bool // qualified func → qualified funcs it calls
}

// fileCtx is the resolution scope of one file: which package it is in and what
// its import aliases mean.
type fileCtx struct {
	a       *analyzer
	pkg     string
	imports map[string]string // local name → import path
}

// importPkg reports the scanned/module package an import alias refers to, or "".
func (f *fileCtx) importPkg(alias string) string { return inModule(f.imports[alias]) }

// typeOf resolves a TYPE expression (a field's or result's declared type) to a
// typeRef: `*Service` → this package's Service, `*husdindex.Indexer` → that
// package's Indexer. Anything else (interfaces, maps, funcs, out-of-module
// types) is deliberately unresolved.
func (f *fileCtx) typeOf(e ast.Expr) typeRef {
	switch x := e.(type) {
	case *ast.StarExpr:
		return f.typeOf(x.X)
	case *ast.Ident:
		if syms := f.a.syms[f.pkg]; syms != nil && syms.types[x.Name] {
			return typeRef{f.pkg, x.Name}
		}
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok {
			if p := f.importPkg(id.Name); p != "" {
				return typeRef{p, x.Sel.Name}
			}
		}
	}
	return typeRef{}
}

// fnCtx is the resolution scope inside one function body: its file's scope plus
// the receiver, the parameters, and the local variables whose type we inferred.
type fnCtx struct {
	*fileCtx
	recv    string             // receiver identifier ("" for a plain func)
	recvT   typeRef            // receiver's type
	locals  map[string]typeRef // local var / parameter → its type
	funcVal map[string]qualRef // local bound to a METHOD VALUE (m := svc.Settle)
}

// qualRef names a function by the package that declares it.
type qualRef struct{ pkg, name string }

// bindParams seeds the parameters into scope. A service arrives as an argument
// at least as often as it is constructed locally (`func h(svc *husdledger.Service)`),
// and the declared type is right there — not inferring it made every such call a
// dead end.
func (f *fnCtx) bindParams(ft *ast.FuncType) {
	if ft == nil || ft.Params == nil {
		return
	}
	for _, p := range ft.Params.List {
		t := f.typeOf(p.Type)
		if t.pkg == "" {
			continue // unresolved (out-of-module, interface, map…) — no guess
		}
		for _, nm := range p.Names {
			f.locals[nm.Name] = t
		}
	}
}

// specTypes resolves one `var`/`const` spec to name → type, for both the typed
// (`var s *husdledger.Service`) and inferred (`var s = husdledger.Default()`) forms.
func (f *fnCtx) specTypes(vs *ast.ValueSpec) map[string]typeRef {
	out := map[string]typeRef{}
	for i, nm := range vs.Names {
		var t typeRef
		switch {
		case vs.Type != nil:
			t = f.typeOf(vs.Type)
		case i < len(vs.Values):
			t = f.valueType(vs.Values[i])
		}
		if t.pkg != "" {
			out[nm.Name] = t
		}
	}
	return out
}

// methodOwner reports the package whose func set owns method m on type t,
// following EMBEDDED types so a promoted method resolves to the type that
// actually declares it. Bounded depth: an embed cycle cannot compile, but the
// analyzer must not hang on malformed input either.
func (f *fnCtx) methodOwner(t typeRef, m string, depth int) string {
	syms := f.a.syms[t.pkg]
	if syms == nil || depth > 8 {
		return ""
	}
	if syms.funcs[m] {
		return t.pkg
	}
	for _, e := range syms.embeds[t.name] {
		if p := f.methodOwner(e, m, depth+1); p != "" {
			return p
		}
	}
	return ""
}

// valueType resolves a VALUE expression to the type of the value it denotes.
// This is the whole point of the resolver: it is what turns `svc.Settle(...)`
// into "husdledger.Settle" instead of a dead end.
func (f *fnCtx) valueType(e ast.Expr) typeRef {
	switch x := e.(type) {
	case *ast.Ident:
		if x.Name == f.recv {
			return f.recvT
		}
		if t, ok := f.locals[x.Name]; ok {
			return t
		}
		if syms := f.a.syms[f.pkg]; syms != nil {
			return syms.globals[x.Name] // package-level var
		}
	case *ast.SelectorExpr:
		// A struct field access: s.indexer → the Service struct's indexer field.
		// Embedded fields are keyed by their type name, so w.Service resolves too.
		base := f.valueType(x.X)
		if syms := f.a.syms[base.pkg]; syms != nil {
			return syms.fields[base.name][x.Sel.Name]
		}
	case *ast.CallExpr:
		return f.resultType(x)
	case *ast.UnaryExpr: // &T{...}
		return f.valueType(x.X)
	case *ast.CompositeLit: // T{...}
		return f.typeOf(x.Type)
	}
	return typeRef{}
}

// resultType reports the type a call evaluates to, so a chained/assigned call
// (`svc := husdledger.Default()`, `husdledger.Default().SyncOnce(ctx)`) resolves.
func (f *fnCtx) resultType(call *ast.CallExpr) typeRef {
	if p, name := f.callee(call); p != "" {
		if syms := f.a.syms[p]; syms != nil {
			return syms.returns[name]
		}
	}
	return typeRef{}
}

// callee reports the package and name of the function a call invokes, resolving
// all three shapes a money sink is reached through:
//
//	foo(...)            same-package call
//	husdledger.Foo(...) import-qualified call
//	svc.Foo(...)        a method on a value whose type we resolved
//
// It returns "" for anything it cannot resolve — an unresolved callee yields no
// edge, so the graph never invents reachability it did not prove.
func (f *fnCtx) callee(call *ast.CallExpr) (pkg, name string) {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		// A local holding a method VALUE (m := svc.Settle; m(ctx)) calls through
		// to the method it was bound to, not to a func in this package.
		if q, ok := f.funcVal[fun.Name]; ok {
			return q.pkg, q.name
		}
		return f.pkg, fun.Name
	case *ast.SelectorExpr:
		// A bare identifier that is an import alias (and is not shadowed by the
		// receiver or a local) qualifies a package-level func.
		if id, ok := fun.X.(*ast.Ident); ok && id.Name != f.recv {
			if _, shadowed := f.locals[id.Name]; !shadowed {
				if p := f.importPkg(id.Name); p != "" {
					return p, fun.Sel.Name
				}
			}
		}
		// Otherwise it is a method on a value: resolve the value's type, then find
		// which package's func set owns the method — which may be an EMBEDDED
		// type's, since embedding promotes methods onto the outer struct.
		return f.methodOwner(f.valueType(fun.X), fun.Sel.Name, 0), fun.Sel.Name
	}
	return "", ""
}

// methodValue reports the function a method-value expression (`svc.Settle`, with
// no call) binds to, so a later `m(ctx)` resolves to it.
func (f *fnCtx) methodValue(e ast.Expr) (qualRef, bool) {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return qualRef{}, false
	}
	if id, ok := sel.X.(*ast.Ident); ok && id.Name != f.recv {
		if _, shadowed := f.locals[id.Name]; !shadowed {
			if p := f.importPkg(id.Name); p != "" {
				return qualRef{p, sel.Sel.Name}, true // pkg.Func as a value
			}
		}
	}
	if p := f.methodOwner(f.valueType(sel.X), sel.Sel.Name, 0); p != "" {
		return qualRef{p, sel.Sel.Name}, true
	}
	return qualRef{}, false
}

// ── the money sinks ─────────────────────────────────────────────────────────
//
// A sink is a MONEY PRIMITIVE recognized by the shape of the code that performs
// it — never by a route or handler name, which is the hand-list this guard
// exists to kill. Each is independently sufficient: reaching ANY of them means a
// function moves or creates money.

// sinkLits are the value types whose CONSTRUCTION is itself a money act — you do
// not build one except to move money with it.
var sinkLits = map[typeRef]string{
	{"util/blockchain", "TokenTransfer"}: "signs an on-chain token movement (treasury mint, org→treasury settle, contributor payout)",
	{"billing/husdindex", "Credit"}:      "projects an on-chain transfer into the commerce ledger as spendable credit",
}

// mintModels are the money MODELS whose CREATION is itself the money act: one of
// these rows coming into existence is spendable balance created or treasury
// disbursed, whatever its field values. Contrast models/transaction, where the
// same row is a Deposit (a mint) or a Withdraw (a debit) depending on its Type —
// so that model is a sink only under the narrower rules above.
//
// The sink is specifically `v := <model>.New(...)` … `v.Create()`. A void
// (New + GetById + Update) never creates the row and is correctly NOT a mint.
var mintModels = map[string]string{
	"models/creditgrant": "a credit grant IS spendable balance; creating one from a client-supplied amount is the C1 mint shape",
	"models/payout":      "a payout disburses the treasury to a client-named destination",
}

// persistVerbs are the datastore methods that WRITE a row. Creating a money
// model through any of them is the same money act; only the verb differs.
var persistVerbs = map[string]bool{"Create": true, "Put": true, "MustPut": true}

// mintModelNew reports whether e constructs a money model (`creditgrant.New(db)`).
func (f *fnCtx) mintModelNew(e ast.Expr) bool {
	for m := range mintModels {
		if isSelectorCall2(e, f.imports, m, "New") {
			return true
		}
	}
	return false
}

// litSink reports whether n constructs a money value type.
func (f *fnCtx) litSink(n ast.Node) bool {
	lit, ok := n.(*ast.CompositeLit)
	if !ok {
		return false
	}
	_, isSink := sinkLits[f.typeOf(lit.Type)]
	return isSink
}

// callSink reports whether n is a call that is itself a money act.
func (f *fnCtx) callSink(n ast.Node) bool {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return false
	}
	// mintauth.WithAuthorized(ctx) — the codebase's OWN assertion of mint
	// authority, and the same capability treasury.Mint and the datastore write
	// sink enforce (mintauth.Require / mintauth.Enforce). Code that elevates a
	// context to mint-authorized is, by its own declaration, on the money path.
	if isSelectorCall(call, f.imports, "mintauth", "WithAuthorized") {
		return true
	}
	// allotment.Grant(...) — mints included monthly credit.
	if isSelectorCall(call, f.imports, "billing/allotment", "Grant") {
		return true
	}
	// <payout cron>.Payout(...) — disburses treasury.
	return isPayoutExecutorCall(call, f.imports)
}

// assignSink reports whether n is an assignment that writes money into a ledger row.
func (f *fnCtx) assignSink(n ast.Node) bool {
	as, ok := n.(*ast.AssignStmt)
	if !ok {
		return false
	}
	// x.Type = transaction.Deposit — a deposit WRITE, not a `== Deposit` read
	// comparison (that is a BinaryExpr, never an AssignStmt RHS).
	for _, rhs := range as.Rhs {
		if isSelector(rhs, f.imports, "models/transaction", "Deposit") {
			return true
		}
	}
	// x.DestinationKind = "iam-user" — mints spendable balance to an IAM user.
	// Precisely the DESTINATION field, so a Withdraw's SourceKind="iam-user" (a
	// debit) is NOT flagged.
	for i, lhs := range as.Lhs {
		if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "DestinationKind" &&
			i < len(as.Rhs) && isStringLit(as.Rhs[i], "iam-user") {
			return true
		}
	}
	// x.Balance += n — a stored balance is INCREASED, which is money created.
	// Precisely the credit direction: engine.ApplyBalanceToInvoice's `-=` (a
	// debit) is not a mint, and a plain `=` initialisation of a fresh row is not
	// either. This is what engine.AdjustCustomerBalance does with a caller-supplied
	// signed amount, so every route reaching it can mint.
	if as.Tok == token.ADD_ASSIGN {
		for _, lhs := range as.Lhs {
			if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "Balance" {
				return true
			}
		}
	}
	return false
}

// mintReachingFuncs returns the set of package-qualified functions whose bodies
// reach a money sink, following resolved calls across the scanned packages
// transitively. Derived from source, so a new handler that mints — directly or
// through a service — is detected automatically.
func mintReachingFuncs(t *testing.T) map[string]bool {
	t.Helper()
	return analyzeFiles(t, parseMintPkgs(t))
}

// parseMintPkgs parses the scanned packages into pkg → files. It is the ONLY
// place source comes from disk, so analyzeFiles below can be exercised against
// synthetic source (TestResolver_CallShapes) without a package on disk.
func parseMintPkgs(t *testing.T) map[string][]*ast.File {
	t.Helper()
	files := map[string][]*ast.File{}
	for _, dir := range mintPkgDirs {
		fset := token.NewFileSet()
		parsed, err := parser.ParseDir(fset, dir, func(fi fs.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", dir, err)
		}
		p := pkgPath(dir)
		for _, pkg := range parsed {
			for _, file := range pkg.Files {
				files[p] = append(files[p], file)
			}
		}
	}
	return files
}

// analyzeFiles runs the sink analysis over pkg → files and returns the set of
// package-qualified functions that reach a money sink.
func analyzeFiles(t *testing.T, files map[string][]*ast.File) map[string]bool {
	t.Helper()

	a := &analyzer{
		syms:    map[string]*pkgSyms{},
		rawSink: map[string]bool{},
		calls:   map[string]map[string]bool{},
	}
	for p := range files {
		a.syms[p] = &pkgSyms{
			types:   map[string]bool{},
			fields:  map[string]map[string]typeRef{},
			returns: map[string]typeRef{},
			funcs:   map[string]bool{},
			embeds:  map[string][]typeRef{},
			globals: map[string]typeRef{},
		}
	}

	// Pass 1 — type declarations, so a local type name resolves.
	for pkg, fs := range files {
		for _, file := range fs {
			for _, decl := range file.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.TYPE {
					continue
				}
				for _, spec := range gd.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						a.syms[pkg].types[ts.Name.Name] = true
					}
				}
			}
		}
	}

	// Pass 2 — struct fields and function results, the facts the resolver reads.
	for pkg, fs := range files {
		for _, file := range fs {
			f := &fileCtx{a: a, pkg: pkg, imports: importAliases(file)}
			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.GenDecl:
					switch d.Tok {
					case token.TYPE:
						for _, spec := range d.Specs {
							ts, ok := spec.(*ast.TypeSpec)
							if !ok {
								continue
							}
							st, ok := ts.Type.(*ast.StructType)
							if !ok {
								continue
							}
							fields := map[string]typeRef{}
							for _, fld := range st.Fields.List {
								t := f.typeOf(fld.Type)
								if len(fld.Names) == 0 {
									// An EMBEDDED field has no name: its type IS the field
									// name, and its methods are PROMOTED onto this struct.
									// Record both so w.Service and the promoted w.Settle()
									// each resolve.
									if t.name != "" {
										fields[t.name] = t
										a.syms[pkg].embeds[ts.Name.Name] = append(a.syms[pkg].embeds[ts.Name.Name], t)
									}
									continue
								}
								for _, nm := range fld.Names {
									fields[nm.Name] = t
								}
							}
							a.syms[pkg].fields[ts.Name.Name] = fields
						}
					}
				case *ast.FuncDecl:
					a.syms[pkg].funcs[d.Name.Name] = true
					if d.Type.Results != nil && len(d.Type.Results.List) > 0 {
						a.syms[pkg].returns[d.Name.Name] = f.typeOf(d.Type.Results.List[0].Type)
					}
				}
			}
		}
	}

	// Pass 2b — package-level vars. Separate from pass 2 because resolving
	// `var svc = husdledger.Default()` reads the `returns` table pass 2 builds,
	// so it can only run once every package's results are known.
	for pkg, fs := range files {
		for _, file := range fs {
			f := &fnCtx{fileCtx: &fileCtx{a: a, pkg: pkg, imports: importAliases(file)}, locals: map[string]typeRef{}}
			for _, decl := range file.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.VAR {
					continue
				}
				for _, spec := range gd.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for name, t := range f.specTypes(vs) {
						a.syms[pkg].globals[name] = t
					}
				}
			}
		}
	}

	// Pass 3 — sinks and call edges.
	for pkg, fs := range files {
		for _, file := range fs {
			fc := &fileCtx{a: a, pkg: pkg, imports: importAliases(file)}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				f := &fnCtx{fileCtx: fc, locals: map[string]typeRef{}, funcVal: map[string]qualRef{}}
				if fn.Recv != nil && len(fn.Recv.List) > 0 {
					if names := fn.Recv.List[0].Names; len(names) > 0 {
						f.recv = names[0].Name
					}
					f.recvT = fc.typeOf(fn.Recv.List[0].Type)
				}
				f.bindParams(fn.Type)
				name := qualify(pkg, fn.Name.Name)
				if _, seen := a.calls[name]; !seen {
					a.calls[name] = map[string]bool{}
				}
				// Per-func state for the GENERIC ledger-create sink: a handler that
				// decodes request input INTO a transaction.Transaction and .Create()s
				// it (so its Type is attacker-controlled, e.g. a Deposit) — the shape
				// the literal `x.Type = Deposit` detector misses. This is exactly
				// api/transaction.Create.
				txVars := map[string]bool{}        // vars assigned from transaction.New(...)
				mintVars := map[string]bool{}      // vars assigned from a mintModel New(...)
				decodeTargets := map[string]bool{} // vars a request body was decoded INTO
				hasCreate := false                 // a .Create() call is present

				ast.Inspect(fn.Body, func(n ast.Node) bool {
					if f.assignSink(n) || f.callSink(n) || f.litSink(n) {
						a.rawSink[name] = true
					}
					switch x := n.(type) {
					case *ast.DeclStmt:
						// `var svc = husdledger.Default()` binds a local exactly as `:=`
						// does; only the syntax differs.
						if gd, ok := x.Decl.(*ast.GenDecl); ok && gd.Tok == token.VAR {
							for _, spec := range gd.Specs {
								if vs, ok := spec.(*ast.ValueSpec); ok {
									for name, t := range f.specTypes(vs) {
										f.locals[name] = t
									}
								}
							}
						}
					case *ast.AssignStmt:
						for i, lhs := range x.Lhs {
							id, ok := lhs.(*ast.Ident)
							if !ok || i >= len(x.Rhs) {
								continue
							}
							// Learn the local's type so a later svc.Method() resolves.
							if tr := f.valueType(x.Rhs[i]); tr.pkg != "" {
								f.locals[id.Name] = tr
							}
							// A method taken as a VALUE (m := svc.Settle) — the call
							// comes later, through m.
							if q, ok := f.methodValue(x.Rhs[i]); ok {
								f.funcVal[id.Name] = q
							}
							if isSelectorCall2(x.Rhs[i], f.imports, "models/transaction", "New") {
								txVars[id.Name] = true
							}
							if f.mintModelNew(x.Rhs[i]) {
								mintVars[id.Name] = true
							}
						}
					case *ast.CallExpr:
						if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
							if sel.Sel.Name == "Create" {
								hasCreate = true
							}
							// v.<persist>() where v is a money-model row — the money act.
							// Every verb that writes the row counts: api/coupon mints a
							// credit grant with MustPut, not Create, so keying on Create
							// alone would miss a real mint the day one moves in here.
							if persistVerbs[sel.Sel.Name] {
								if id, ok := sel.X.(*ast.Ident); ok && mintVars[id.Name] {
									a.rawSink[name] = true
								}
							}
						}
						if tgt := decodeTargetIdent(x); tgt != "" {
							decodeTargets[tgt] = true
						}
						// A resolved callee inside the scanned set → a graph edge.
						if p, callee := f.callee(x); p != "" {
							if _, scanned := a.syms[p]; scanned {
								a.calls[name][qualify(p, callee)] = true
							}
						}
					}
					return true
				})
				// Generic ledger-create sink: request body decoded INTO a
				// transaction.New(...) var that is then .Create()d.
				if hasCreate {
					for v := range decodeTargets {
						if txVars[v] {
							a.rawSink[name] = true
							break
						}
					}
				}
			}
		}
	}

	// Transitive closure: a func reaches a sink if it is a raw sink or calls a
	// func that reaches one.
	reaches := map[string]bool{}
	for f := range a.rawSink {
		reaches[f] = true
	}
	for changed := true; changed; {
		changed = false
		for f, callees := range a.calls {
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

// isSelectorCall2 reports whether e is a call expression `alias.sel(...)` where
// alias imports a path ending in pkgSuffix (accepts an ast.Expr, e.g. an
// AssignStmt RHS, so `v := transaction.New(...)` can be recognized).
func isSelectorCall2(e ast.Expr, imports map[string]string, pkgSuffix, sel string) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	return isSelectorCall(call, imports, pkgSuffix, sel)
}

// decodeTargetIdent returns the identifier a request-body decode call
// deserializes INTO (its last argument), or "" if call is not a recognized
// decode. Recognizes json.Decode/DecodeBytes/Unmarshal(..., v) and gin's
// Bind/ShouldBind*/BindJSON(&v) — the ways a handler populates a value from
// untrusted input. Used to catch a handler that decodes straight into a ledger
// transaction and Creates it (dynamic, attacker-controlled Type).
func decodeTargetIdent(call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	switch sel.Sel.Name {
	case "Decode", "DecodeBytes", "Unmarshal", "Bind", "BindJSON", "ShouldBind", "ShouldBindJSON", "ShouldBindWith":
	default:
		return ""
	}
	if len(call.Args) == 0 {
		return ""
	}
	arg := call.Args[len(call.Args)-1]
	if u, ok := arg.(*ast.UnaryExpr); ok { // &v
		arg = u.X
	}
	if id, ok := arg.(*ast.Ident); ok {
		return id.Name
	}
	return ""
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
	"api/billing.Topup":                 "credits ONLY the amount the caller's own saved card was charged (money-in == credit); own subject",
	"api/billing.TopupWithToken":        "credits ONLY the amount the caller's own card nonce was charged (money-in == credit); own subject",
	"api/billing.GrantAllotment":        "amount is clamped to the caller's REAL subscription via planForGrant unless MayMintMoney (TestAllotment_OrgAdminCannotInflatePlan)",
	"api/billing.RunAllotments":         "per-user amount is subscription-derived in grantOrgAllotments; NO client-supplied amount or plan",
	"api/billing.HandleProviderWebhook": "unauthenticated by design — trust anchor is the per-provider signature, not a commerce token; not an org-admin surface",
}

// methodGatedMintHandlers reach a sink but gate it INSIDE the handler per-method
// (not via route middleware), each verified by a named companion test.
var methodGatedMintHandlers = map[string]string{
	"api/billing.ZapDispatch": "mint methods (billing.deposit) gated per-method on middleware.MayMintMoney (TestZapDeposit_OrgAdminDenied / TestZapReads_OrgAdminNotBlocked)",
	"api/transaction.Create":  "api/transaction.Create gates the MINT case (Deposit / credit-to-iam-user) on middleware.MayMintMoney inside the handler — org-admin deposit→403 (api/transaction TestCreate_OrgAdminDepositToIAMUser_Denied); non-mint Withdraw/Transfer stay org-admin. mintauth.Enforce at the sink is the fail-closed backstop.",
}

type mintRoute struct{ method, path, handler string }

// TestMintSurface_EveryMintRouteGatedOrProvablyUserSafe is the enumeration guard.
func TestMintSurface_EveryMintRouteGatedOrProvablyUserSafe(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")

	reaches := mintReachingFuncs(t)

	// Detector self-check: the known-critical mint functions MUST be flagged, or
	// the enumeration is silently under-detecting (which is the whole failure mode
	// we are guarding against). "Create" is api/transaction.Create — the GENERIC
	// ledger-create sink (decodes request input INTO a transaction.New var and
	// .Create()s it), which the literal `Type=Deposit` detector alone misses; if it
	// is not flagged the generic-sink detector is broken.
	// The husdledger/husdindex/treasury entries are the cross-package half: each
	// is reached ONLY by resolving a call onto a service value, which is exactly
	// what the same-package-only call graph could not do.
	for _, must := range []string{
		"api/billing.Deposit", "api/billing.Refund", "api/billing.GrantAllotment",
		"api/billing.zapDeposit", "api/billing.ZapDispatch", "api/billing.Credit",
		"api/affiliate.executePayouts", "api/transaction.Create",
		"api/billing.SyncHUSD", "api/billing.SettleHUSD", "api/billing.MigrateHUSD",
		"billing/husdledger.SyncOnce", "billing/husdledger.Settle", "billing/husdledger.Migrate",
		"billing/husdledger.MintCredit", "billing/husdindex.Sync", "treasury.Mint",
		"api/billing.CreateCreditGrant", "api/billing.CreatePayout",
		"api/billing.AdjustCustomerBalance", "api/billing.ReconcileInboundTransfer",
		"billing/engine.AdjustCustomerBalance",
	} {
		if !reaches[must] {
			t.Fatalf("mint-sink detector did NOT flag %q — the source enumeration is broken; fix the detector before trusting this guard", must)
		}
	}
	// Negative self-check: the read-only husd surface must NOT be flagged, or the
	// detector is over-approximating (flagging a whole package instead of the
	// funcs that actually mint) and every "GATED" line below is worthless.
	// billing/engine.ApplyBalanceToInvoice is the direction check: it writes the
	// SAME Balance field as AdjustCustomerBalance but decreases it (`-=`), so a
	// detector that flagged it would be reading "touches money" as "mints money"
	// and would drag every invoice-collection route into the mint surface.
	for _, mustNot := range []string{
		"api/billing.StatusHUSD", "billing/husdledger.Config", "billing/husdledger.Enabled",
		"billing/engine.ApplyBalanceToInvoice", "api/billing.VoidCreditGrant",
	} {
		if reaches[mustNot] {
			t.Fatalf("mint-sink detector flagged read-only %q — the enumeration is over-approximating; a guard that flags everything proves nothing", mustNot)
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

// registeredMintRoutes registers the real billing + affiliate Route()s and returns
// every registered route whose terminal handler reaches a mint sink.
func registeredMintRoutes(t *testing.T, reaches map[string]bool) []mintRoute {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	rr := newRecordingRouter(app)
	routeMintSurface(rr.Group("/v1"))

	var out []mintRoute
	for _, ri := range *rr.rec {
		// ri.handler is the runtime's fully-qualified name
		// (github.com/hanzoai/commerce/api/billing.SyncHUSD); the graph is keyed by
		// the module-relative form, so the two meet with no name guessing.
		if qual := strings.TrimPrefix(ri.handler, modulePath+"/"); reaches[qual] {
			out = append(out, mintRoute{ri.method, ri.path, qual})
		}
	}
	return out
}

// orgAdminEngine registers billing + affiliate behind a seed that mints an ORG-admin
// identity (org-level isAdmin, NOT a SuperAdmin) — the exact C1 adversary.
func orgAdminEngine(t *testing.T) *zip.App {
	t.Helper()
	eng := zip.New(zip.Config{DisableStartupMessage: true})
	eng.Use(zip.H(func(c *zip.Ctx) error {
		c.Locals("iam_authenticated", true)
		c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
		c.Locals("iam_claims", &auth.IAMClaims{Owner: "acme", IsAdmin: true})
		return c.Next()
	}))
	v1 := eng.Group("/v1")
	routeMintSurface(v1)
	return eng
}

func probe(eng *zip.App, method, path string) int {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := eng.Test(req)
	if err != nil {
		return 0
	}
	return resp.StatusCode
}

// concretePath replaces `:param` / `*param` segments with a literal so the
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
