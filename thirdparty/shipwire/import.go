package shipwire

import (
	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/util/json"
)

func Import(db *datastore.Datastore, filename string) {
	for record := range json.Iterator(filename) {
		if config.IsDevelopment && record.Index > 25 {
			break // Only import first 25 in development
		}

	}
}
