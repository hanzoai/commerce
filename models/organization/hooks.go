package organization

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/app"
	"github.com/hanzoai/commerce/models/namespace"
	"github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/secret"
	"github.com/hanzoai/commerce/util/rand"
)

const (
	DefaultAppName   = "Hanzo App"
	DefaultStoreName = "Default"
)

// Hooks
func (o *Organization) BeforeCreate() error {
	// Last line of defense against persisting a credential as a tenant.
	// Callers are expected to reject a bearer-shaped selector before they ever
	// reach provisioning (pkg/org.Resolve, api/billing.webhooks), but this is
	// the one chokepoint EVERY create passes through — orm.Model.CreateCtx
	// aborts the write on a BeforeCreate error — so no present or future caller
	// can mint an org named from an API key, whatever it does upstream. A token
	// persisted as an org name is both a credential leak (incident 2026-07-02)
	// and unbounded tenant cardinality: one 5.25KB org per distinct bearer.
	if secret.Like(o.Name) {
		return ErrSecretLikeName
	}

	o.Fees.Id = o.Id()
	if o.SecretKey == nil {
		o.SecretKey = []byte(rand.SecretKey())
	}
	// Generate Tokens
	o.AddDefaultTokens()

	return nil
}

// OnSaved runs after an org is persisted, with the org's name. pkg/org
// registers cache invalidation here so a mutation is not masked by the resolve
// TTL. It is a seam rather than a direct call because pkg/org depends on this
// package, so this package cannot import it back.
var OnSaved = func(name string) {}

// AfterUpdate is the one place every full-entity org write passes through
// (orm.Model.UpdateCtx dispatches it), so invalidating here covers Update and
// MustUpdate from any handler without each call site having to remember.
func (o *Organization) AfterUpdate(prev *Organization) error {
	OnSaved(o.Name)
	// A rename must also drop the OLD name, or it keeps resolving from cache
	// to an org that no longer answers to it.
	if prev != nil && prev.Name != o.Name {
		OnSaved(prev.Name)
	}
	return nil
}

func (o *Organization) AfterCreate() error {
	// Save namespace so we can decode keys for this organization later
	db := datastore.New(o.Context())
	ns := namespace.New(db)
	err := ns.GetOrCreate("Name=", o.Name)

	if err != nil {
		log.Warn("Failed to put namespace: %v", err)
	}

	ns.Name = o.Name
	ns.IntId = o.Key().IntID()
	ns.MustUpdate()

	nsCtx := o.Namespaced(o.Context())
	nsDb := datastore.New(nsCtx)

	ap := app.New(nsDb)
	ap.Name = DefaultAppName
	ap.MustCreate()

	stor := store.New(nsDb)
	stor.Name = DefaultStoreName
	stor.Currency = o.Currency
	stor.MustCreate()

	o.DefaultApp = ap.Id()
	o.DefaultStore = stor.Id()

	o.MustUpdate()

	return nil
}
