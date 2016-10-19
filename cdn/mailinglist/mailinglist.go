package mailinglist

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mailinglist"
	"crowdstart.com/util/log"
)

func Js(c *gin.Context) {
	id := c.Params.ByName("mailinglistid")
	db := datastore.New(c)

	ml := mailinglist.New(db)

	// Set key and namespace correctly
	ml.SetKey(id)
	log.Debug("mailinglist: %v", ml)
	log.Debug("key: %v", ml.Key())
	log.Debug("namespace: %v", ml.Key().Namespace())
	ml.SetNamespace(ml.Key().Namespace())

	if err := ml.Get(nil); err != nil {
		c.String(404, fmt.Sprintf("Failed to retrieve mailing list '%v': %v", id, err))
		return
	}

	c.Writer.Header().Add("Content-Type", "application/javascript")
	c.String(200, ml.Js())
}
