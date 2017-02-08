package organization

import (
	"hanzo.io/datastore"
	"hanzo.io/models/app"
	"hanzo.io/models/namespace"
	"hanzo.io/util/event"
	"hanzo.io/util/log"
	"hanzo.io/util/rand"
)

const (
	DefaultAppName = "Store"
)

// Hooks
func (o *Organization) BeforeCreate() error {
	o.SecretKey = []byte(rand.SecretKey())

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

	return nil
}

// Emit on update, as an organization may care about when it's updated.
func (o *Organization) AfterUpdate(previous *Organization) error {
	// url := config.UrlFor("api", "/organization/", o.Id(), "a", "js")
	// if err := cloudflare.Purge(o.Context(), url); err != nil {
	// 	log.Error("Failed to purge site %v", err, o.Context())
	// }
	// url = config.UrlFor("api", "/organization/", o.Id(), "n", "js")
	// if err := cloudflare.Purge(o.Context(), url); err != nil {
	// 	log.Error("Failed to purge organization %v", err, o.Context())
	// }
	return event.Emit(o.Context(), o.Name, "organization.updated", o)
}

// Doesn't make sense to emit these
// func (o *Organization) AfterCreate() error {
// 	return event.Emit(o.Context(), o.Name, "organization.created", o)
// }

// func (o *Organization) AfterDelete() error {
// 	return event.Emit(o.Context(), o.Name, "organization.deleted", o)
// }
