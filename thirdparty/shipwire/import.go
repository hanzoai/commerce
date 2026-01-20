package shipwire

import (
	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/json"
)

func Import(db *datastore.Datastore, filename string) {
	for record := range json.Iterator(filename) {
		if config.IsDevelopment && record.Index > 25 {
			break // Only import first 25 in development
		}

	}
}
