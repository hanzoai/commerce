package models

import "crowdstart.io/datastore"

func GetNamespaces(c interface{}) []string {
	namespaces := make([]string, 0)
	db := datastore.New(c)
	keys, err := db.Query("__namespace__").KeysOnly().GetAll(nil)
	if err != nil {
		panic(err)
	}

	for _, k := range keys {
		namespaces = append(namespaces, k.StringID())
	}

	return namespaces
}
