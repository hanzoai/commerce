package fixtures

import (
	"appengine"
	"appengine/delay"

	"crowdstart.io/datastore"
	"crowdstart.io/thirdparty/indiegogo"
	"crowdstart.io/util/log"
)

var contributors = delay.Func("fixtures-contributors", func(c appengine.Context) {
	db := datastore.New(c)

	if count, _ := db.Query("user").Count(c); count > 10 {
		log.Debug("Contributor fixtures already loaded, skipping.")
		return
	}

	indiegogo.ImportCSV(db, "resources/contributions.csv")
})
