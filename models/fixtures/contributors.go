package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/thirdparty/indiegogo"
	"crowdstart.io/util/log"
	"crowdstart.io/util/task"
)

var contributors = task.Func("fixtures-contributors", func(c *gin.Context) {
	db := datastore.New(c)

	if count, _ := db.Query("user").Count(db.Context); count > 10 {
		log.Debug("Contributor fixtures already loaded, skipping.")
		return
	}

	indiegogo.ImportCSV(db, "resources/contributions.csv")
})
