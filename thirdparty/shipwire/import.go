package shipwire

import (
	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/util/json"
)

func Import(db *datastore.Datastore, filename string) {
	for record := range json.Iterator(filename) {
		if config.IsDevelopment && record.Index > 25 {
			break // Only import first 25 in development
		}

	}
}
