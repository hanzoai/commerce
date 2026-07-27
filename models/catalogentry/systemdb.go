package catalogentry

import (
	"context"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/nscontext"
)

// systemNamespace is the platform-global namespace the catalog lives in — not a
// per-tenant org. Brand partitioning is by the entry's Brand field.
//
// Unexported on purpose: SystemDB is the one way to reach the store, so a caller
// cannot half-remember the namespace string and read an empty catalog.
const systemNamespace = "system"

// SystemDB returns a datastore scoped to the platform catalog namespace.
//
// Namespacing here is by CONTEXT, which is the only namespacing the SQL layer
// honors — not datastore.NewNamespaced, which fail-closes to a nil DB for
// reserved namespaces. A nil DB reads as "the catalog is empty", and code that
// prices things off an empty catalog reports zero cost, so this distinction is
// worth the comment.
//
// A nil ctx is treated as Background rather than panicking: callers reach the
// catalog from request handlers, from cron, and from process start, and only the
// first has a ctx to pass.
func SystemDB(ctx context.Context) *datastore.Datastore {
	if ctx == nil {
		ctx = context.Background()
	}
	return datastore.New(nscontext.WithNamespace(ctx, systemNamespace))
}
