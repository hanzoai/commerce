// Package deploy is the ONE way the PaaS control plane (platform.hanzo.ai /
// console) provisions a commercially-deployable OSS Hanzo product for a tenant
// org with prepaid billing already wired. It has two pieces:
//
//   - Catalog: the declared inventory of products we can resell as managed
//     cloud, each with its billable unit expressed as a meter price spec.
//   - Render: turns a (Product, Tenant) pair into an operator `hanzo.ai/v1`
//     Service CR — the product container plus the metering reverse-proxy
//     sidecar — so the product is gated+metered the instant it comes up.
//
// This is deliberately data + a pure function: the control plane calls
// Render and applies the YAML (kubectl/operator). No new API surface, no
// tRPC/gRPC — the deploy path is "emit a CR, let the operator reconcile",
// the same lifecycle every other Hanzo service already uses.
package deploy

import "fmt"

// Unit is the human-readable billable unit of a product — what the customer
// is actually paying for. It is documentation that rides with the catalog so
// console/hanzo.ai can render "billed per <Unit>" without a second source.
type Unit string

const (
	UnitRequest  Unit = "request"        // priced per API request
	UnitVector   Unit = "vector op"      // upsert/search against a collection
	UnitDocument Unit = "document"       // indexed document
	UnitRecord   Unit = "record write"   // row/record persisted (base)
	UnitSeat     Unit = "seat"           // per active user (handled out-of-band)
	UnitStorage  Unit = "GB-month"       // stored bytes (handled by a sweeper)
	UnitCompute  Unit = "compute-second" // execution time (functions)
)

// Product is one commercially-deployable OSS Hanzo product. The fields are
// exactly what Render needs to emit a metered Service CR, plus the descriptive
// billing metadata the storefront shows.
type Product struct {
	// Name is the operator service / image name, e.g. "vector". The per-tenant
	// CR is named "<Name>-<orgSlug>".
	Name string

	// Image is the GHCR repository, e.g. "ghcr.io/hanzoai/vector".
	Image string

	// Port is the product's own listen port inside the pod (the meter-proxy
	// forwards to 127.0.0.1:Port). e.g. 6333 (vector), 7700 (search).
	Port int

	// Provider is the usage label recorded to commerce (defaults to Name).
	Provider string

	// Unit is the billable unit, for display.
	Unit Unit

	// Prices is the meter price-table spec (see metering/proxy.ParsePriceTable):
	// the per-request charge policy that encodes the billable unit. This is the
	// product's price list and the single place its cost model lives.
	Prices string

	// SkipPaths are path prefixes the meter never gates/charges (health, metrics).
	SkipPaths []string

	// StorageMount is the in-container path the product persists to; when set
	// the CR gets a per-tenant PVC mounted there. Empty = stateless.
	StorageMount string

	// StorageSize is the per-tenant PVC size (e.g. "2Gi"). Required iff
	// StorageMount is set.
	StorageSize string

	// Env is product-specific container env the CR must carry (e.g. Qdrant's
	// HTTP/GRPC port settings). Billing/identity env is added by Render.
	Env map[string]string

	// Meterable reports whether the product is billed via the request-path
	// meter (proxy). Some units (seats, storage, compute) are metered by a
	// separate sweeper/recorder, not the per-request gate — those products are
	// still deployable but Render emits no sidecar gate for them.
	Meterable bool
}

func (p Product) provider() string {
	if p.Provider != "" {
		return p.Provider
	}
	return p.Name
}

// Catalog is the inventory of commercially-deployable products, keyed by Name.
// It is the single source of truth for "what we can resell and how it's billed".
// Adding a product here makes it deployable+billable; no other code changes.
var Catalog = map[string]Product{
	// Vector DB (Qdrant fork). Billed per vector operation: writes (upserts)
	// cost more than reads (searches). Per-tenant storage isolates collections.
	"vector": {
		Name:         "vector",
		Image:        "ghcr.io/hanzoai/vector",
		Port:         6333,
		Unit:         UnitVector,
		Prices:       "POST|/collections|2 ; PUT|/collections|2 ; *|/collections|1 ; default:0",
		SkipPaths:    []string{"/healthz", "/readyz", "/metrics", "/livez"},
		StorageMount: "/qdrant/storage",
		StorageSize:  "2Gi",
		Env: map[string]string{
			"QDRANT__SERVICE__HTTP_PORT": "6333",
			"QDRANT__SERVICE__GRPC_PORT": "6334",
		},
		Meterable: true,
	},

	// Full-text search (Meilisearch fork). Billed per indexed document (writes)
	// and per search query. Per-tenant storage isolates indexes.
	"search": {
		Name:         "search",
		Image:        "ghcr.io/hanzoai/search",
		Port:         7700,
		Unit:         UnitDocument,
		Prices:       "POST|/indexes/|3 ; PUT|/indexes/|3 ; POST|/multi-search|1 ; POST|/indexes|1 ; default:1",
		SkipPaths:    []string{"/health", "/metrics", "/version"},
		StorageMount: "/meili_data",
		StorageSize:  "2Gi",
		Env:          map[string]string{"MEILI_HTTP_ADDR": "0.0.0.0:7700"},
		Meterable:    true,
	},

	// Base (PocketBase fork): per-tenant SQLite backend. Billed per record
	// write (create/update/delete). Reads are free to encourage adoption.
	// Base hosts its own metering via a core.ServeEvent hook (its router is not
	// net/http), so the proxy sidecar is NOT used — Meterable=false here means
	// "do not attach the proxy"; billing is in-process. Storage is per-tenant.
	"base": {
		Name:         "base",
		Image:        "ghcr.io/hanzoai/base",
		Port:         8090,
		Unit:         UnitRecord,
		Prices:       "", // metered in-process by base's hook, not the proxy
		SkipPaths:    []string{"/api/health"},
		StorageMount: "/data",
		StorageSize:  "2Gi",
		Meterable:    false,
	},
}

// Lookup returns the catalog entry for a product name.
func Lookup(name string) (Product, error) {
	p, ok := Catalog[name]
	if !ok {
		return Product{}, fmt.Errorf("deploy: unknown product %q (not in catalog)", name)
	}
	return p, nil
}

// Names returns the catalog product names (unordered).
func Names() []string {
	out := make([]string, 0, len(Catalog))
	for n := range Catalog {
		out = append(out, n)
	}
	return out
}
