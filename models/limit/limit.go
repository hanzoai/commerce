// Copyright © 2026 Hanzo AI. MIT License.

// A limit is one bound a person set on what an org, or every holder of a plan,
// may hold at once: how many sandboxes run together, and the bounds that join it.
//
// The value a limit takes where nobody set one is the platform's, compiled into
// the code that enforces it. A row here is a person's decision over that value,
// made in admin.hanzo.ai and recorded on the audit trail by the op that wrote
// it, so an empty table changes nothing. Which names may be set, and the range
// each may take, is the platform's registry, never this package's: the store
// holds what was decided, and the caller that decides refuses what it does not
// know.
//
// It lives beside the plan authority, in the same platform-global namespace,
// because a limit is resolved the way a plan is read: the org's own row first,
// then its plan's, then the code's.
package limit

import (
	"context"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/orm"
)

// Namespace is the platform-global namespace limits live in: the SAME "system"
// namespace the plan authority and the rate book use. A limit is set centrally
// by admin.hanzo.ai and read for every tenant, never stored per tenant.
const Namespace = "system"

// AuthorityDB returns a datastore scoped to the limit store. Pass the request
// context so tracing and deadlines flow; a nil context degrades to Background.
func AuthorityDB(ctx context.Context) *datastore.Datastore {
	if ctx == nil {
		ctx = context.Background()
	}
	return datastore.New(nscontext.WithNamespace(ctx, Namespace))
}

func init() { orm.Register[Limit]("limit") }

// Limit is one bound, set by a person, on one scope.
type Limit struct {
	mixin.Model[Limit]

	// Slug is the row's identity, "<scope>/<name>", derived by Bind and never
	// typed. One field, because that is what the store is queried by.
	Slug string `json:"slug"`

	// Name is which bound this is: a key of the platform's registry, such as
	// "sandbox.running".
	//
	// It is not called Key: mixin.Entity requires a Key() method, and a field of
	// that name shadows it, which leaves Query() with a nil entity and Get()
	// nil-panicking far from the cause.
	Name string `json:"name"`

	// Scope is whom the bound holds for: "org:<org>" for one org, "plan:<slug>"
	// for every holder of one plan.
	Scope string `json:"scope"`

	// Value is the bound: the most allowed, 0 for none, -1 for no bound at all.
	Value int64 `json:"value"`

	// Reason is why the person set it.
	Reason string `json:"reason"`

	// UpdatedBy is who set it last.
	UpdatedBy string `json:"updatedBy"`
}

var _ mixin.Entity = (*Limit)(nil)

// Bind derives the slug from the scope and the name, so the identity a write
// stores and the address a read asks by cannot disagree.
func (m *Limit) Bind() { m.Slug = m.Scope + "/" + m.Name }

// New returns an empty limit bound to db.
func New(db *datastore.Datastore) *Limit {
	m := new(Limit)
	m.Init(db)
	return m
}

// Query returns a limit query against db.
func Query(db *datastore.Datastore) datastore.Query { return db.Query("limit") }
