package datastore

import "context"

func GetNamespaces(ctx context.Context) []string {
	namespaces := make([]string, 0)

	// Fetch namespaces from special __namespace__ table
	db := New(ctx)
	keys, err := db.Query("__namespace__").GetKeys()
	if err != nil {
		panic(err)
	}

	// Append stringID's
	for _, k := range keys {
		namespaces = append(namespaces, k.StringID())
	}

	return namespaces
}

func GetKinds(ctx context.Context) []string {
	kinds := make([]string, 0)
	db := New(ctx)
	keys, err := db.Query("__kind__").GetKeys()
	if err != nil {
		panic(err)
	}

	for _, k := range keys {
		kinds = append(kinds, k.StringID())
	}

	return kinds
}
