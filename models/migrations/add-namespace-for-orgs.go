package migrations

import (
	"appengine"

	"crowdstart.io/models/constants"
	"crowdstart.io/models/namespace"
	"crowdstart.io/models/organization"
	"crowdstart.io/util/log"

	ds "crowdstart.io/datastore"
)

var _ = New("add-namespace-for-orgs",
	NoSetup,
	func(db *ds.Datastore, org *organization.Organization) {
		nsCtx, err := appengine.Namespace(db.Context, constants.NamespaceNamespace)
		if err != nil {
			log.Error("Could not update namespace %v", err, db.Context)
		}

		nsDb := ds.New(nsCtx)
		ns := namespace.New(nsDb)

		rootKey := nsDb.NewKey(ns.Kind(), constants.NamespaceRootKey, 0, nil)
		key := nsDb.NewKey(ns.Kind(), "", 0, rootKey)

		ns.StringId = org.Name
		ns.SetKey(key)
		ns.GetOrCreate("StringId=", ns.StringId)
		ns.IntId = org.Key().IntID()
		ns.MustPut()
	},
)
